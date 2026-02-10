// Package host provides the plugin host for llmd. It discovers plugins
// (compiled extensions and Yaegi dynamic plugins), builds a command
// table, and dispatches command execution to the owning plugin.
package host

import (
	"fmt"
	"log"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/plugin"
	"github.com/jpl-au/llmd/sdk"
)

// Host manages plugins and command execution.
type Host struct {
	store    *llmd.Store
	commands map[string]*cmdEntry
	plugins  []sdk.Plugin
}

type cmdEntry struct {
	cmd      sdk.Command
	plugin   sdk.Plugin
	isPlugin bool // true for yaegi plugins, false for extensions
}

// New creates a Host, loading all available plugins. When store is nil,
// the Host can still enumerate commands (for help/discovery) but cannot
// execute them — sdk.API remains nil. When store is non-nil, New sets
// the global sdk.API so plugins can access the store.
//
// Plugin loading: compiled extensions (registered via init()) are loaded
// first, then Yaegi dynamic plugins from user directories. Yaegi load
// errors are logged but don't prevent startup — a broken user plugin
// should not take down the entire CLI.
func New(store *llmd.Store) *Host {
	h := &Host{
		store:    store,
		commands: make(map[string]*cmdEntry),
	}

	// Set the global store handle so plugins can call sdk.API.Read(), etc.
	if store != nil {
		sdk.API = newAPI(store)
	}

	// Compiled extensions (e.g. cli package) registered at init() time.
	for _, ext := range extension.All() {
		h.addPlugin(ext.Plugin(), false)
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
	sdk.Dispatch = h.Exec
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

// Exec dispatches a command to its owning plugin. It builds an
// sdk.Context from the author and stdin, then delegates to the
// plugin's Exec method. Returns sdk.ErrUnknownCmd if cmd is not
// registered.
func (h *Host) Exec(cmd string, args []string, author string, stdin []byte) (sdk.Response, error) {
	entry, ok := h.commands[cmd]
	if !ok {
		return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
	}

	ctx := sdk.Context{Author: author, Stdin: stdin}
	return entry.plugin.Exec(ctx, cmd, args)
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
