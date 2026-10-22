// Package host provides the command host for llmd. It discovers compiled
// extensions, builds a command table, and dispatches command execution
// to the owning extension.
package host

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	igit "github.com/jpl-au/llmd/internal/git"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/telemetry"
	"github.com/jpl-au/llmd/internal/term"
	"github.com/jpl-au/llmd/internal/validate"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// Host manages extensions and command execution. It is the central
// orchestrator: it discovers compiled extensions, builds a unified
// command table, and dispatches execution to the owning extension. For
// each request it builds an [sdk.Context] carrying store bridges bound
// to that request, so extensions access the store without direct
// dependencies and without package-level state. Host implements
// [sdk.Runtime] and hands itself to commands on the same context.
type Host struct {
	store      *llmd.Store
	lim        validate.Limits
	git        sdk.GitStore
	commands   map[string]*cmdEntry
	extensions []sdk.Extension
}

// cmdEntry maps a command to its owning extension.
type cmdEntry struct {
	cmd sdk.Command
	ext sdk.Extension
}

// New creates a Host without a store. The Host can enumerate commands
// (for help/discovery) but cannot execute store operations - the store
// bridges on each [sdk.Context] remain nil.
func New() *Host {
	return setup(nil)
}

// Open creates a Host backed by a store at the given path (or the
// default path if empty). The caller must defer Close() to release
// the underlying database connection.
func Open(dbPath string) (*Host, error) {
	store, err := llmd.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return setup(store), nil
}

// Close closes the underlying store. Safe to call on a storeless host.
func (h *Host) Close() error {
	if h.store != nil {
		return h.store.Close()
	}
	return nil
}

// Dir returns the project directory that holds the .llmd store, or ""
// for a storeless host.
func (h *Host) Dir() string {
	if h.store == nil {
		return ""
	}
	return filepath.Dir(h.store.Dir())
}

// Dispatch implements [sdk.Runtime] by executing a command through
// this host. It is Exec under the name commands see.
func (h *Host) Dispatch(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (sdk.Response, error) {
	return h.Exec(ctx, cmd, args, author, stdin, dbPath)
}

// Subscribe implements [sdk.Runtime]. The callback receives every
// store event synchronously on the emitting goroutine. A no-op when
// the host has no store.
func (h *Host) Subscribe(fn func(pkgevents.Event)) {
	if h.store == nil {
		return
	}
	h.store.Bus().Subscribe(&callbackHandler{fn: fn})
}

// Init implements [sdk.Runtime]. It creates a new store at dbPath (or
// the default path when empty), writes the default .llmd/.gitignore,
// and returns the path created.
func (h *Host) Init(dbPath string) (string, error) {
	store, err := llmd.Init(dbPath)
	if err != nil {
		return "", err
	}
	path := store.Path()
	if err := store.Close(); err != nil {
		return "", fmt.Errorf("closing store: %w", err)
	}

	// Create the default whitelist .gitignore beside the new database.
	if err := config.InitGitignoreIn(filepath.Dir(path)); err != nil {
		return "", fmt.Errorf("creating gitignore: %w", err)
	}

	return path, nil
}

// Context builds the [sdk.Context] a command receives: the caller's
// context, the author, this host as the runtime, and store bridges
// bound to ctx so cancellation and timeouts propagate to every store
// operation. The bridges are nil on a storeless host.
func (h *Host) Context(ctx context.Context, author string) sdk.Context {
	sctx := sdk.Context{
		Context: ctx,
		Author:  author,
		Runtime: h,
		Git:     h.git,
	}
	if h.store != nil {
		sctx.Documents = newDocumentAPI(h.store, h.lim, ctx)
		sctx.Tasks = newTaskAPI(h.store, h.lim, h.git, ctx)
		sctx.Links = newLinkAPI(h.store, h.lim, ctx)
		sctx.Tags = newTagAPI(h.store, h.lim, ctx)
		sctx.Audits = newAuditAPI(h.store, ctx)
		sctx.Queue = newQueueAPI(h.store, ctx)
		sctx.Activities = newActivityAPI(h.store, ctx)
		sctx.Mirror = newMirrorAPI(h.store, ctx)
		sctx.Agents = newAgentAPI(h.store, h.git, ctx)
		sctx.Rules = newRuleAPI(h.store.Dir())
		sctx.Hooks = newHookAPI(h.store, h.lim, h.git, ctx)
	}
	return sctx
}

// RunOpts carries the parameters for a single command invocation.
type RunOpts struct {
	Cmd    string
	Args   []string
	Author string // from --author flag; resolved from config if empty
	Stdin  []byte
	DB     string
}

// Run looks up the command, opens a store if needed, resolves the
// author, executes, and cleans up. It is the main entry point for
// CLI invocations - callers provide parsed flags and get back a
// response.
func (h *Host) Run(ctx context.Context, opts RunOpts) (sdk.Response, error) {
	emit := func(err error) {
		e := telemetry.Entry{
			Source:  "cli",
			Command: opts.Cmd,
			Args:    opts.Args,
			Author:  opts.Author,
			Success: err == nil,
		}
		if err != nil {
			e.Error = err.Error()
		}
		telemetry.Emit(e)
	}

	c := h.commands[opts.Cmd]
	if c == nil {
		err := fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, opts.Cmd)
		emit(err)
		return nil, err
	}

	// Open the store. Commands that can run without one (init,
	// version, hook init, ...) carry on when it is missing; streaming
	// commands (mcp, serve) start anyway and report per call; every
	// other command fails. The store is wired into a fresh host for
	// this call so the receiver is never a half-built host.
	target := h
	if h.store == nil {
		store, err := llmd.Open(opts.DB)
		switch {
		case err == nil:
			defer func() {
				if cerr := store.Close(); cerr != nil {
					slog.Warn("closing store", "err", cerr)
				}
			}()
			target = setup(store)
		case h.storeless(opts.Cmd):
			slog.Debug("running without store", "cmd", opts.Cmd, "error", err)
		case c.cmd.Streams:
			slog.Warn("starting without store", "cmd", opts.Cmd, "error", err)
			telemetry.Emit(telemetry.Entry{
				Source:  "cli",
				Event:   "start.nostore",
				Command: opts.Cmd,
				Success: true,
				Error:   err.Error(),
			})
		default:
			emit(err)
			return nil, err
		}
	}

	author, err := target.resolveAuthor(opts.Author, c.cmd.NeedsAuthor)
	if err != nil {
		emit(err)
		return nil, err
	}

	resp, err := target.Exec(ctx, opts.Cmd, opts.Args, author, opts.Stdin, opts.DB)
	emit(err)
	return resp, err
}

// storeless reports whether the command can run without a store.
func (h *Host) storeless(cmd string) bool {
	for _, ext := range extension.All() {
		if sl, ok := ext.(extension.Storeless); ok {
			if slices.Contains(sl.NoStoreCommands(), cmd) {
				return true
			}
		}
	}
	return false
}

// resolveAuthor determines the author identity for a command. The
// --author flag takes precedence. For interactive terminals, the
// configured author is used as fallback. Non-interactive callers
// must always provide --author explicitly.
func (h *Host) resolveAuthor(flag string, required bool) (string, error) {
	if flag != "" {
		return flag, nil
	}

	// Fall back to config author for interactive terminals.
	if term.Interactive() {
		cfg, err := config.Load()
		if err != nil {
			slog.Warn("reading config for author", "err", err)
		}
		if cfg.Author != "" {
			return cfg.Author, nil
		}
	}

	if !required {
		return "", nil
	}

	// Author is required but missing - return a helpful error.
	if term.Interactive() {
		return "", fmt.Errorf("%w: author not configured - run 'llmd config author \"Your Name\"' or pass --author", sdk.ErrMissingArg)
	}
	return "", fmt.Errorf("%w: --author is required for non-interactive use", sdk.ErrMissingArg)
}

// setup loads extensions and configures the command table. Called by
// both New() and Open(). When store is non-nil it also bridges internal
// bus events to extension EventHandlers and the pipeline handler.
func setup(store *llmd.Store) *Host {
	// Load validation limits from config.
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("loading config for validation limits", "err", err)
	}
	lim := validate.LoadLimits(cfg.Limits)

	h := &Host{
		store:    store,
		lim:      lim,
		git:      igit.New(),
		commands: make(map[string]*cmdEntry),
	}

	// Compiled extensions (e.g. cli package) registered at init() time.
	for _, ext := range extension.All() {
		h.addExtension(ext)
	}

	// Pipeline handler: auto-spawn agents when tasks enter columns
	// with pipeline configuration.
	if store != nil {
		store.Bus().Subscribe(&pipelineHandler{
			dir:   store.Dir(),
			agent: newAgentAPI(store, h.git, context.Background()),
		})
	}

	// Wire extension event handlers to the internal bus so extensions
	// can react to document changes.
	if store != nil {
		var handlers []extension.EventHandler
		for _, ext := range extension.All() {
			if eh, ok := ext.(extension.EventHandler); ok {
				handlers = append(handlers, eh)
			}
		}
		if len(handlers) > 0 {
			ctx := extension.NewContext(store, store.DB(), nil)
			store.Bus().Subscribe(&eventBridge{handlers: handlers, ctx: ctx})
		}
	}

	return h
}

// addExtension registers an extension's commands.
func (h *Host) addExtension(ext sdk.Extension) {
	h.extensions = append(h.extensions, ext)
	for _, cmd := range ext.Commands() {
		h.commands[cmd.Name] = &cmdEntry{cmd: cmd, ext: ext}
	}
}

// Exec dispatches a command to its owning extension. It creates fresh
// per-request bridge instances bound to the given context, populates
// an sdk.Context with them, and delegates to the extension's Exec method.
// Returns sdk.ErrUnknownCmd if cmd is not registered.
func (h *Host) Exec(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (sdk.Response, error) {
	entry, ok := h.commands[cmd]
	if !ok {
		return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
	}

	if entry.cmd.NeedsAuthor && author == "" {
		return nil, fmt.Errorf("%w: author not configured", sdk.ErrMissingArg)
	}

	// Commands that need a store get a clear error when none is open
	// rather than a nil pointer panic. This happens when a streaming
	// server (mcp, serve) started without a .llmd directory and a
	// tool call arrives.
	if h.store == nil && !h.storeless(cmd) && !entry.cmd.Streams {
		return nil, fmt.Errorf("store not initialised - run llmd init")
	}

	sctx := h.Context(ctx, author)
	sctx.Stdin = stdin
	sctx.DBPath = dbPath

	resp, err := entry.ext.Exec(sctx, cmd, args)

	// After a storeless command succeeds (e.g. init), try to open the
	// store so subsequent calls on this host have a working connection.
	if err == nil && h.store == nil {
		if store, serr := llmd.Open(dbPath); serr == nil {
			*h = *setup(store)
			telemetry.Emit(telemetry.Entry{
				Source:  "cli",
				Event:   "connect",
				Command: cmd,
				Success: true,
			})
		}
	}

	return resp, err
}

// Commands returns a copy of all registered commands, keyed by name.
func (h *Host) Commands() map[string]*sdk.Command {
	cmds := make(map[string]*sdk.Command)
	for name, e := range h.commands {
		cmd := e.cmd
		cmds[name] = &cmd
	}
	return cmds
}

// Extensions returns all loaded extensions in registration order.
func (h *Host) Extensions() []sdk.Extension {
	return h.extensions
}

// callbackHandler adapts a plain function to the internal bus Handler
// interface. Used by Host.Subscribe so consumers can subscribe without
// importing internal packages.
type callbackHandler struct {
	fn func(pkgevents.Event)
}

func (h *callbackHandler) HandleEvent(_ context.Context, e pkgevents.Event) error {
	h.fn(e)
	return nil
}
