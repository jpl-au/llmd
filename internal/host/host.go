// Package host provides the plugin host for llmd. It discovers plugins
// (compiled extensions and Yaegi dynamic plugins), builds a command
// table, and dispatches command execution to the owning plugin.
package host

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	igit "github.com/jpl-au/llmd/internal/git"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/plugin"
	"github.com/jpl-au/llmd/internal/validate"
	"github.com/jpl-au/llmd/sdk"
)

// Host manages plugins and command execution. It is the central
// orchestrator: it discovers plugins from both compiled extensions and
// Yaegi dynamic sources, builds a unified command table, and dispatches
// execution to the owning plugin. The Host also wires up the SDK domain
// globals (sdk.Documents, sdk.Tasks, etc.) so that plugins can access
// the store without direct dependencies.
type Host struct {
	store    *llmd.Store
	lim      validate.Limits
	commands map[string]*cmdEntry
	plugins  []sdk.Plugin
}

// cmdEntry maps a command to its owning plugin and tracks the plugin's
// origin. The isPlugin flag distinguishes Yaegi dynamic plugins from
// compiled extensions — this distinction drives help output grouping
// (core commands vs plugin commands) and the "plugins" command listing.
type cmdEntry struct {
	cmd      sdk.Command
	plugin   sdk.Plugin
	isPlugin bool // true for yaegi plugins, false for extensions
}

// New creates a Host without a store. The Host can enumerate commands
// (for help/discovery) but cannot execute store operations — the SDK
// domain globals remain nil.
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

// setup wires up the SDK globals, loads plugins, and configures the
// command table. Called by both New() and Open().
//
// When store is non-nil, it sets sdk.Documents, sdk.Tasks, sdk.Links,
// sdk.Tags, and sdk.Activities so plugins can access the store. It also
// bridges internal bus events to extension EventHandlers.
//
// Plugin loading: compiled extensions (registered via init()) are loaded
// first, then Yaegi dynamic plugins from user directories. Yaegi load
// errors are logged but don't prevent startup — a broken user plugin
// should not take down the entire CLI.
func setup(store *llmd.Store) *Host {
	// Load validation limits from config.
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("loading config for validation limits", "err", err)
	}
	lim := validate.LoadLimits(cfg)

	h := &Host{
		store:    store,
		lim:      lim,
		commands: make(map[string]*cmdEntry),
	}

	// Store-independent globals — always available.
	sdk.Git = igit.New()
	sdk.Config = config.Store{}

	// Set domain globals with a background context. These are used
	// by tests and Yaegi plugins. CLI commands use per-request
	// bridges from sdk.Context instead.
	bg := context.Background()
	if store != nil {
		sdk.Documents = newDocumentAPI(store, lim, bg)
		sdk.Tasks = newTaskAPI(store, lim, bg)
		sdk.Links = newLinkAPI(store, lim, bg)
		sdk.Tags = newTagAPI(store, lim, bg)
		sdk.Activities = newActivityAPI(store, bg)
		sdk.Mirror = newMirrorAPI(store, bg)
	}

	// Compiled extensions (e.g. cli package) registered at init() time.
	for _, ext := range extension.All() {
		h.addPlugin(ext.Plugin(), false)
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

	// Dynamic plugins from .llmd/plugins/ and ~/.llmd/plugins/.
	// Errors are logged, not returned — partial plugin loading is
	// better than failing entirely.
	yaegiPlugins, err := plugin.Load()
	if err != nil {
		log.Printf("yaegi: %v", err)
	}
	for _, p := range yaegiPlugins {
		h.addPlugin(p, true)
	}

	// Set sdk function vars so commands can discover and dispatch.
	sdk.Init = func(dbPath string) (string, error) {
		store, err := llmd.Init(dbPath)
		if err != nil {
			return "", err
		}
		path := store.Path()
		store.Close()

		// Create default .llmd/.gitignore for new stores.
		if err := config.InitGitignore(); err != nil {
			return "", fmt.Errorf("creating gitignore: %w", err)
		}

		return path, nil
	}
	sdk.Dispatch = func(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (sdk.Response, error) {
		return h.Exec(ctx, cmd, args, author, stdin, dbPath)
	}
	sdk.AllCommands = h.Commands
	sdk.PluginNames = h.pluginNames

	return h
}

// addPlugin registers a plugin's commands. isPlugin distinguishes yaegi
// plugins (true) from compiled extensions (false) for help output grouping.
func (h *Host) addPlugin(p sdk.Plugin, isPlugin bool) {
	h.plugins = append(h.plugins, p)
	for _, cmd := range p.Commands() {
		h.commands[cmd.Name] = &cmdEntry{cmd: cmd, plugin: p, isPlugin: isPlugin}
	}
}

// pluginNames returns the names of loaded yaegi plugins.
func (h *Host) pluginNames() []string {
	var names []string
	seen := make(map[string]bool)
	for _, e := range h.commands {
		if e.isPlugin && !seen[e.plugin.Name()] {
			seen[e.plugin.Name()] = true
			names = append(names, e.plugin.Name())
		}
	}
	return names
}

// Exec dispatches a command to its owning plugin. It creates fresh
// per-request bridge instances bound to the given context, populates
// an sdk.Context with them, and delegates to the plugin's Exec method.
// Returns sdk.ErrUnknownCmd if cmd is not registered.
func (h *Host) Exec(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (sdk.Response, error) {
	entry, ok := h.commands[cmd]
	if !ok {
		return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
	}

	if entry.cmd.NeedsAuthor && author == "" {
		return nil, fmt.Errorf("%w: author not configured", sdk.ErrMissingArg)
	}

	sctx := sdk.Context{
		Context: ctx,
		Author:  author,
		Stdin:   stdin,
		DBPath:  dbPath,
		Git:     sdk.Git,
		Config:  sdk.Config,
	}

	// Create per-request bridges bound to the caller's context so
	// cancellation and timeouts propagate to store operations.
	if h.store != nil {
		sctx.Documents = newDocumentAPI(h.store, h.lim, ctx)
		sctx.Tasks = newTaskAPI(h.store, h.lim, ctx)
		sctx.Links = newLinkAPI(h.store, h.lim, ctx)
		sctx.Tags = newTagAPI(h.store, h.lim, ctx)
		sctx.Activities = newActivityAPI(h.store, ctx)
		sctx.Mirror = newMirrorAPI(h.store, ctx)
	}

	return entry.plugin.Exec(sctx, cmd, args)
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

// ExtCommands returns commands from compiled extensions only.
func (h *Host) ExtCommands() map[string]*sdk.Command {
	cmds := make(map[string]*sdk.Command)
	for name, e := range h.commands {
		if !e.isPlugin {
			cmd := e.cmd
			cmds[name] = &cmd
		}
	}
	return cmds
}

// PluginCommands returns commands from yaegi plugins only.
func (h *Host) PluginCommands() map[string]*sdk.Command {
	cmds := make(map[string]*sdk.Command)
	for name, e := range h.commands {
		if e.isPlugin {
			cmd := e.cmd
			cmds[name] = &cmd
		}
	}
	return cmds
}

// Plugins returns all loaded plugins in registration order
// (compiled extensions first, then Yaegi plugins).
func (h *Host) Plugins() []sdk.Plugin {
	return h.plugins
}
