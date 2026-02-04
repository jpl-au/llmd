// Package host provides the plugin host for llmd.
package host

import (
	"fmt"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

// Host manages plugins and command execution.
type Host struct {
	store    *llmd.Store
	commands map[string]*cmdEntry
	plugins  []sdk.Plugin
}

type cmdEntry struct {
	cmd    sdk.Command
	plugin sdk.Plugin
}

// New creates a new Host.
func New(store *llmd.Store) *Host {
	h := &Host{
		store:    store,
		commands: make(map[string]*cmdEntry),
	}

	if store != nil {
		sdk.API = newAPI(store)
	}

	// Register plugins from extensions
	for _, ext := range extension.All() {
		h.register(ext.Plugin())
	}

	return h
}

func (h *Host) register(p sdk.Plugin) {
	h.plugins = append(h.plugins, p)
	for _, cmd := range p.Commands() {
		h.commands[cmd.Name] = &cmdEntry{cmd: cmd, plugin: p}
	}
}

// Exec executes a command.
func (h *Host) Exec(cmd string, args []string, author string, stdin []byte) (sdk.Result, error) {
	entry, ok := h.commands[cmd]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}

	ctx := sdk.Context{Author: author, Stdin: stdin}
	return entry.plugin.Exec(ctx, cmd, args)
}

// Commands returns all registered commands.
func (h *Host) Commands() map[string]*sdk.Command {
	cmds := make(map[string]*sdk.Command)
	for name, e := range h.commands {
		cmd := e.cmd
		cmds[name] = &cmd
	}
	return cmds
}

// Plugins returns loaded plugins.
func (h *Host) Plugins() []sdk.Plugin {
	return h.plugins
}
