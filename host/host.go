// Package host provides the command host for llmd. It discovers compiled
// extensions, builds a command table, and dispatches command execution
// to the owning extension.
package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/jpl-au/llmd/errs"
	docpath "github.com/jpl-au/llmd/path"

	"github.com/jpl-au/llmd/clock"
	"github.com/jpl-au/llmd/config"
	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/tasks"
	"github.com/jpl-au/llmd/extension"
	igit "github.com/jpl-au/llmd/git"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
	"github.com/jpl-au/llmd/sqlite"
	"github.com/jpl-au/llmd/telemetry"
	"github.com/jpl-au/llmd/term"
	"github.com/jpl-au/llmd/validate"
)

// Host manages extensions and command execution. It is the central
// orchestrator: it discovers compiled extensions, builds a unified
// command table, and dispatches execution to the owning extension. For
// each request it builds an [sdk.Context] carrying store bridges bound
// to that request, so extensions access the store without direct
// dependencies and without package-level state. Host implements
// [sdk.Runtime] and hands itself to commands on the same context.
type Host struct {
	mu           sync.RWMutex // guards store, which attach may set once
	store        *engine.Store
	lim          validate.Limits
	limSet       bool
	taskOrder    tasks.Order   // ordering within a column; "" reads the config
	retention    time.Duration // how long queue messages are kept; see retentionSet
	retentionSet bool          // retention was set by an option, not the config
	hookLimit    int           // most pending messages the session-start hook prints; see hookLimitSet
	hookLimitSet bool          // hookLimit was set by an option, not the config
	worktree     bool          // spawned agents get their own git worktree; from agent.worktree
	harness      string        // the harness this computer runs agents under; from agent.harness
	git          sdk.GitStore
	commands     map[string]*cmdEntry
	extensions   []sdk.Extension
	opts         []Option                // reapplied to hosts built for a single call
	subs         []func(pkgevents.Event) // Subscribe callbacks, for Watch and attach
}

// Option configures a Host. Options are applied by New and Open and
// carried over to any host built for a single request, so a setting
// made once holds for every call.
type Option func(*Host)

// WithGit sets the git implementation. The default runs the git
// command line in the working directory.
func WithGit(g sdk.GitStore) Option {
	return func(h *Host) { h.git = g }
}

// WithLimits sets the validation limits for paths and content. The
// default comes from the limits section of the configuration file.
func WithLimits(l validate.Limits) Option {
	return func(h *Host) {
		h.lim = l
		h.limSet = true
	}
}

// WithTaskOrder sets how tasks are ordered within a column, for the
// board, review and task next alike: tasks.OrderPriority or
// tasks.OrderPosition. The default comes from the task.order key of
// the configuration file, which is priority.
func WithTaskOrder(o tasks.Order) Option {
	return func(h *Host) { h.taskOrder = o }
}

// WithQueueRetention sets how long queue messages are kept before
// vacuum removes them; zero keeps them forever. The default comes from
// the queue.retention key of the configuration file, which is 30 days.
func WithQueueRetention(d time.Duration) Option {
	return func(h *Host) {
		h.retention = d
		h.retentionSet = true
	}
}

// WithHookLimit sets how many pending queue messages the session-start
// hook prints into an agent's context; zero prints them all. The
// default comes from the hook.limit key of the configuration file,
// which is 10.
func WithHookLimit(n int) Option {
	return func(h *Host) {
		h.hookLimit = n
		h.hookLimitSet = true
	}
}

// cmdEntry maps a command to its owning extension.
type cmdEntry struct {
	cmd sdk.Command
	ext sdk.Extension
}

// New creates a Host without a store. The Host can enumerate commands
// (for help/discovery) but cannot execute store operations - the store
// bridges on each [sdk.Context] remain nil.
func New(opts ...Option) *Host {
	return setup(nil, opts...)
}

// storeOptions opens stores with the database.* settings from the
// config. A config that cannot be read leaves the defaults; main has
// already warned about it.
func storeOptions() []engine.Option {
	cfg, _ := config.Load()
	return []engine.Option{engine.WithDatabase(DatabaseSettings(cfg))}
}

// DatabaseSettings returns the SQLite settings the config asks for,
// with defaults in place of any value that is missing or invalid.
func DatabaseSettings(cfg config.Config) sqlite.Settings {
	mmap := cfg.CountSetting("database.mmap_size", 0)
	return sqlite.Settings{
		BusyTimeout: cfg.DurationSetting("database.busy_timeout"),
		CacheMB:     cfg.CountSetting("database.cache_size", 1),
		MMapMB:      &mmap,
	}
}

// Open creates a Host backed by a store at the given path (or the
// default path if empty). The caller must defer Close() to release
// the underlying database connection.
func Open(dbPath string, opts ...Option) (*Host, error) {
	store, err := engine.Open(dbPath, storeOptions()...)
	if err != nil {
		return nil, err
	}
	return setup(store, opts...), nil
}

// Open implements [sdk.Runtime]. It opens the store at dbPath as a host
// of its own, with this host's options and extensions, so the store is
// used exactly as this one is.
func (h *Host) Open(dbPath string) (sdk.Store, error) {
	other, err := Open(dbPath, h.opts...)
	switch {
	case errors.Is(err, engine.ErrNoStore):
		return nil, errs.Mark(sdk.ErrNotFound, err)
	case err != nil:
		return nil, err
	}
	return other, nil
}

// Close closes the underlying store. Safe to call on a storeless host.
func (h *Host) Close() error {
	if store := h.current(); store != nil {
		return store.Close()
	}
	return nil
}

// current returns the attached store, or nil.
func (h *Host) current() *engine.Store {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.store
}

// attach gives a storeless host a store and wires the store's event
// bus to the pipeline handler and to extension event handlers. It
// happens at most once; a host that already has a store closes the
// new one and keeps its own. This is how a streaming server started
// before "init" picks the store up afterwards.
func (h *Host) attach(store *engine.Store) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.store != nil {
		if err := store.Close(); err != nil {
			slog.Warn("closing duplicate store", "err", err)
		}
		return
	}
	h.store = store

	if err := store.Tasks.SetOrder(h.taskOrder); err != nil {
		slog.Warn("task order", "order", h.taskOrder, "err", err)
	}
	store.SetQueueRetention(h.retention)

	// Callbacks subscribed while the host had no store join the bus now.
	for _, fn := range h.subs {
		store.Bus().Subscribe(&callbackHandler{fn: fn})
	}

	// Pipeline handler: auto-spawn agents when tasks enter columns
	// with pipeline configuration.
	store.Bus().Subscribe(&pipelineHandler{
		dir:   store.Dir(),
		agent: newAgentAPI(store, h.git, h.worktree, h.harness, context.Background()),
	})

	// Wire extension event handlers to the internal bus so extensions
	// can react to document changes.
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

// Dir returns the project directory that holds the .llmd store, or ""
// for a storeless host.
func (h *Host) Dir() string {
	store := h.current()
	if store == nil {
		return ""
	}
	return filepath.Dir(store.Dir())
}

// Dispatch implements [sdk.Runtime] by executing a command through
// this host. It is Exec under the name commands see.
func (h *Host) Dispatch(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (sdk.Response, error) {
	return h.Exec(ctx, cmd, args, author, stdin, dbPath)
}

// Subscribe implements [sdk.Runtime]. The callback receives every
// store event synchronously on the emitting goroutine, and every
// store.changed event Watch raises. A callback subscribed before the
// host has a store joins the bus when one is attached.
func (h *Host) Subscribe(fn func(pkgevents.Event)) {
	h.mu.Lock()
	h.subs = append(h.subs, fn)
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return
	}
	store.Bus().Subscribe(&callbackHandler{fn: fn})
}

// Watch implements [sdk.Runtime]. A host without a store waits, polling
// at the interval, for one to be attached: a server started before
// "init" picks the store up and watches it from then on. Changes made
// by other processes are
// delivered to the Subscribe callbacks only, as store.changed events:
// they never enter the internal bus, so the search index, the message
// queue and extensions see only the typed events of this process.
func (h *Host) Watch(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("watch: interval must be positive")
	}
	store := h.current()
	for store == nil {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
		store = h.current()
	}
	return store.Watch(ctx, interval, func(db string) {
		e := pkgevents.Event{
			Type:      pkgevents.StoreChanged,
			Timestamp: clock.Now(),
			Metadata:  map[string]any{"db": db},
		}
		h.mu.RLock()
		subs := slices.Clone(h.subs)
		h.mu.RUnlock()
		for _, fn := range subs {
			fn(e)
		}
	})
}

// Init implements [sdk.Runtime]. It creates a new store at dbPath (or
// the default path when empty), writes the default .llmd/.gitignore,
// and returns the path created.
func (h *Host) Init(dbPath string) (string, error) {
	store, err := engine.Init(dbPath, storeOptions()...)
	if errors.Is(err, engine.ErrStoreExists) {
		// Nothing is touched, the .gitignore included: rewriting it
		// would drop the patterns config git allow added.
		path, _ := docpath.ResolveDB(dbPath)
		return path, errs.Mark(sdk.ErrExists, err)
	}
	if err != nil {
		return "", err
	}
	path := store.Path()
	if err := store.Close(); err != nil {
		return "", fmt.Errorf("closing store: %w", err)
	}

	// Create the default whitelist .gitignore beside the new database,
	// and allow this store's mirror: the default one commits llmd/,
	// so a named store (llmd-docs.db) adds llmd-docs/.
	if err := config.InitGitignoreIn(filepath.Dir(path)); err != nil {
		return "", fmt.Errorf("creating gitignore: %w", err)
	}
	if err := config.GitAllowIn(filepath.Dir(path), mirrorName(path)+"/"); err != nil {
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
	if store := h.current(); store != nil {
		sctx.Documents = newDocumentAPI(store, h.lim, ctx)
		sctx.Tasks = newTaskAPI(store, h.lim, h.git, ctx)
		sctx.Links = newLinkAPI(store, h.lim, ctx)
		sctx.Tags = newTagAPI(store, h.lim, ctx)
		sctx.Audits = newAuditAPI(store, ctx)
		sctx.Queue = newQueueAPI(store, ctx)
		sctx.Activities = newActivityAPI(store, ctx)
		sctx.Mirror = newMirrorAPI(store, ctx)
		sctx.Agents = newAgentAPI(store, h.git, h.worktree, h.harness, ctx)
		sctx.Rules = newRuleAPI(store.Dir(), func(name string) error {
			_, err := store.Agents.Agent(ctx, name)
			return err
		})
		sctx.Hooks = newHookAPI(store, h.lim, h.git, h.hookLimit, ctx)
	}
	return sctx
}

// RunOpts carries the parameters for a single command invocation.
type RunOpts struct {
	Cmd    string
	Args   []string
	Author string    // from --author flag; resolved from config if empty
	Stdin  io.Reader // the CLI's piped stdin, or nil; read only if the command asks
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
		err := sdk.Errorf(sdk.ErrUnknownCmd, "llmd: unknown command %s. Run llmd --help to list them.", opts.Cmd)
		emit(err)
		return nil, err
	}

	// Open the store. Commands that can run without one (init,
	// version, hook init, ...) carry on when it is missing; streaming
	// commands (mcp, serve) start anyway and report per call; every
	// other command fails. The store is wired into a fresh host for
	// this call so the receiver is never a half-built host.
	target := h
	if h.current() == nil {
		store, err := engine.Open(opts.DB, storeOptions()...)
		switch {
		case err == nil:
			defer func() {
				if cerr := store.Close(); cerr != nil {
					slog.Warn("closing store", "err", cerr)
				}
			}()
			target = setup(store, h.opts...)
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
			// The command could not start: say which, as its own
			// errors do. A missing store is "not found" to callers.
			err = fmt.Errorf("%s: %w", opts.Cmd, err)
			if errors.Is(err, engine.ErrNoStore) {
				err = errs.Mark(sdk.ErrNotFound, err)
			}
			emit(err)
			return nil, err
		}
	}

	author, err := target.resolveAuthor(opts.Cmd, opts.Author, c.cmd.NeedsAuthor)
	if err != nil {
		emit(err)
		return nil, err
	}

	resp, err := target.exec(ctx, opts.Cmd, opts.Args, author, opts.Stdin, opts.DB, term.Interactive())
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
func (h *Host) resolveAuthor(cmd, flag string, required bool) (string, error) {
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
		return "", sdk.Errorf(sdk.ErrMissingArg, "%s: no author is set. Run llmd config author \"Your Name\", or pass --author.", cmd)
	}
	return "", sdk.AuthorRequired(cmd)
}

// setup loads extensions and configures the command table. Called by
// both New() and Open(). When store is non-nil it also bridges internal
// bus events to extension EventHandlers and the pipeline handler.
func setup(store *engine.Store, opts ...Option) *Host {
	h := &Host{
		commands: make(map[string]*cmdEntry),
		opts:     opts,
	}
	for _, o := range opts {
		o(h)
	}

	// The configuration supplies anything the options did not set.
	// agent.worktree has no option, so the configuration always loads.
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("loading config", "err", err)
	}
	if h.git == nil {
		h.git = igit.New(cfg.DurationSetting("git.timeout"))
	}
	if !h.limSet {
		h.lim = validate.LoadLimits(cfg)
	}
	if h.taskOrder == "" {
		h.taskOrder = tasks.Order(cfg.Task.Order)
	}
	if !h.retentionSet {
		d, err := config.Retention(cfg.Queue.Retention)
		if err != nil {
			slog.Warn("queue.retention", "value", cfg.Queue.Retention, "err", err)
			d, _ = config.Retention(config.Defaults().Queue.Retention)
		}
		h.retention = d
		h.retentionSet = true
	}
	if !h.hookLimitSet {
		h.hookLimit = cfg.HookLimit()
		h.hookLimitSet = true
	}
	h.worktree = cfg.AgentWorktree()
	h.harness = cfg.Agent.Harness

	// Compiled extensions (e.g. cli package) registered at init() time.
	for _, ext := range extension.All() {
		h.addExtension(ext)
	}

	if store != nil {
		h.attach(store)
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
// stdin is the request content (MCP, HTTP); nil or empty means none.
// Returns sdk.ErrUnknownCmd if cmd is not registered.
func (h *Host) Exec(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (sdk.Response, error) {
	var in io.Reader
	if len(stdin) > 0 {
		in = bytes.NewReader(stdin)
	}
	return h.exec(ctx, cmd, args, author, in, dbPath, false)
}

// exec is Exec with the input as a reader, so the CLI can hand over
// its stdin without reading it.
func (h *Host) exec(ctx context.Context, cmd string, args []string, author string, stdin io.Reader, dbPath string, tty bool) (sdk.Response, error) {
	entry, ok := h.commands[cmd]
	if !ok {
		return nil, sdk.Errorf(sdk.ErrUnknownCmd, "llmd: unknown command %s. Run llmd --help to list them.", cmd)
	}

	if entry.cmd.NeedsAuthor && author == "" {
		return nil, sdk.AuthorRequired(cmd)
	}

	// Commands that need a store get a clear error when none is open
	// rather than a nil pointer panic. This happens when a streaming
	// server (mcp, serve) started without a .llmd directory and a
	// tool call arrives.
	if h.current() == nil && !h.storeless(cmd) && !entry.cmd.Streams {
		return nil, fmt.Errorf("%s: there is no store here. Run llmd init to create one.", cmd)
	}

	sctx := h.Context(ctx, author)
	sctx.Stdin = stdin
	sctx.TTY = tty
	sctx.DBPath = dbPath

	resp, err := entry.ext.Exec(sctx, cmd, args)

	// After a storeless command succeeds (e.g. init), try to open the
	// store so subsequent calls on this host have a working connection.
	if err == nil && h.current() == nil {
		if store, serr := engine.Open(dbPath, storeOptions()...); serr == nil {
			h.attach(store)
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
