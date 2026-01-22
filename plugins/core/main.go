//go:build wasip1

// Package main is the core plugin for llmd.
//
// The core plugin provides all standard llmd commands for document operations:
//   - cat: Read and display document content
//   - ls: List documents in the store
//   - write: Create or update documents
//   - grep: Search documents using regular expressions
//
// This plugin is embedded in the llmd binary but can be overridden by placing
// a core.wasm file in .llmd/plugins/. This allows customisation of core
// behaviour without recompiling the main binary.
//
// # Building
//
// The plugin must be built as a reactor WASM module:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o core.wasm .
//
// The -buildmode=c-shared flag creates a reactor module that initialises but
// doesn't exit, allowing the host to call the plugin's exported functions.
//
// # Registration
//
// Plugin registration happens in init() rather than main() because c-shared
// mode doesn't execute main(). The init() function calls sdk.Register() to
// register the plugin with the host.
package main

import (
	"github.com/jpl-au/llmd/plugins/core/commands"
	"github.com/jpl-au/llmd/sdk"
)

// init registers the plugin with the host.
// In c-shared mode, main() is not called, so registration must happen here.
func init() {
	sdk.Register(&CorePlugin{})
}

// main is required for compilation but not called in c-shared mode.
func main() {
	// Not executed - c-shared modules use _initialize, not _start
}

// CorePlugin implements the standard llmd commands.
//
// This plugin provides the fundamental commands that all llmd installations
// need. It demonstrates best practices for plugin development and can serve
// as a reference implementation for custom plugins.
type CorePlugin struct{}

// Manifest returns the core plugin's metadata and commands.
//
// The manifest declares the plugin's identity (name, version, author) and
// lists all commands it provides. Each command can optionally be exposed
// via MCP (Model Context Protocol) for AI assistant integration.
func (p *CorePlugin) Manifest() sdk.Manifest {
	return sdk.Manifest{
		Name:        "core",
		Version:     "1.0.0",
		Author:      "jpl-au",
		Description: "Core llmd commands",
		Commands: []sdk.Command{
			commands.Cat,
			commands.Edit,
			commands.Grep,
			commands.Ls,
			commands.Rm,
			commands.Write,
		},
	}
}

// ExecuteCommand routes a command to the appropriate handler.
//
// The host calls this method when a command registered by this plugin is
// invoked. The ctx provides execution context (interface, author, format),
// args contains positional arguments, and flags contains parsed flag values.
//
// Each command is implemented in a separate file in the commands package
// for maintainability and separation of concerns.
func (p *CorePlugin) ExecuteCommand(ctx sdk.Context, cmd string, args []string, flags map[string]any) (sdk.Result, error) {
	switch cmd {
	case "cat":
		return commands.ExecCat(ctx, args, flags)
	case "edit":
		return commands.ExecEdit(ctx, args, flags)
	case "grep":
		return commands.ExecGrep(ctx, args, flags)
	case "ls":
		return commands.ExecLs(ctx, args, flags)
	case "rm":
		return commands.ExecRm(ctx, args, flags)
	case "write":
		return commands.ExecWrite(ctx, args, flags)
	default:
		return nil, sdk.ErrUnknownCommand{Command: cmd}
	}
}
