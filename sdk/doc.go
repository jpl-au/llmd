//go:build wasip1

/*
Package sdk provides the plugin SDK for llmd.

This guide explains how to create plugins for llmd. Plugins extend llmd with
custom commands that can be invoked from the CLI or via MCP (Model Context
Protocol) for AI assistant integration.

# Quick Start

1. Create a new Go module for your plugin:

	mkdir myplugin && cd myplugin
	go mod init github.com/yourname/myplugin

2. Add the SDK dependency:

	go get github.com/jpl-au/llmd/sdk

3. Create main.go:

	//go:build wasip1

	package main

	import "github.com/jpl-au/llmd/sdk"

	func init() {
		sdk.Register(&MyPlugin{})
	}

	func main() {}

	type MyPlugin struct{}

	func (p *MyPlugin) Manifest() sdk.Manifest {
		return sdk.Manifest{
			Name:        "myplugin",
			Version:     "1.0.0",
			Description: "My custom plugin",
			Commands: []sdk.Command{
				{
					Name:        "hello",
					Description: "Say hello",
					Usage:       "hello [name]",
					MCPEnabled:  true,
				},
			},
		}
	}

	func (p *MyPlugin) ExecuteCommand(ctx sdk.Context, cmd string, args []string, flags map[string]any) (sdk.Result, error) {
		switch cmd {
		case "hello":
			name := "world"
			if len(args) > 0 {
				name = args[0]
			}
			return sdk.TextResult("Hello, " + name + "!"), nil
		default:
			return nil, sdk.ErrUnknownCommand{Command: cmd}
		}
	}

4. Build the plugin:

	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o myplugin.wasm .

5. Install the plugin:

	cp myplugin.wasm ~/.llmd/plugins/    # Global install
	cp myplugin.wasm .llmd/plugins/      # Project-local install

# Architecture Overview

Plugins are WebAssembly modules that run inside the llmd host. The host loads
plugins at startup, initialises them, and routes commands to the appropriate
plugin.

Communication between host and plugin uses Protocol Buffers. The SDK handles
all the serialisation details - you just implement the Plugin interface.

# Plugin Interface

Every plugin must implement the Plugin interface:

	type Plugin interface {
		Manifest() Manifest
		ExecuteCommand(ctx Context, cmd string, args []string, flags map[string]any) (Result, error)
	}

Optionally, plugins can also implement:

  - EventHandler: Receive notifications when documents change
  - Shutdowner: Perform cleanup when the plugin is unloaded

# Registration

Plugins must register themselves in init(), not main(). This is because plugins
are built with -buildmode=c-shared, which creates "reactor" WASM modules where
main() is never called.

	func init() {
		sdk.Register(&MyPlugin{})
	}

	func main() {
		// Required for compilation, but never executed
	}

# Host API

Plugins can access the llmd document store through the sdk.Host variable:

	// Read a document
	content, err := sdk.Host.Read("docs/readme")

	// Write a document
	err := sdk.Host.Write("docs/new", []byte("content"), "author", "commit message")

	// List documents
	paths, err := sdk.Host.List("docs/")

	// Search documents
	results, err := sdk.Host.Search("query")

	// Grep with full-text search
	matches, err := sdk.Host.Grep("query")

	// Delete a document (soft delete)
	err := sdk.Host.Delete("docs/old", "author")

# Commands

Commands are defined in the Manifest and executed via ExecuteCommand. Each
command has:

  - Name: The command name (e.g., "cat", "ls")
  - Description: Shown in help text
  - Usage: Syntax hint (e.g., "cat <path>")
  - Flags: Optional flags the command accepts
  - MCPEnabled: Whether to expose via MCP
  - MCPName: Override the MCP tool name

# Flags

Flags provide optional parameters to commands:

	sdk.Command{
		Name: "search",
		Flags: []sdk.Flag{
			{Name: "limit", Short: "n", Type: "int", Default: "10", Description: "Max results"},
			{Name: "case-insensitive", Short: "i", Type: "bool", Description: "Ignore case"},
		},
	}

Supported types: "string", "int", "bool", "stringSlice"

# Results

Commands return either TextResult or JSONResult:

	// Plain text output
	return sdk.TextResult("Operation completed"), nil

	// Structured JSON output
	return sdk.JSONResult{Data: map[string]any{
		"count": 42,
		"items": items,
	}}, nil

# Events

Plugins can subscribe to document store events by listing them in the Manifest:

	sdk.Manifest{
		SubscribedEvents: []string{
			sdk.EventDocumentWritten,
			sdk.EventDocumentDeleted,
		},
	}

Then implement the EventHandler interface:

	func (p *MyPlugin) HandleEvent(event sdk.Event) error {
		if event.Type == sdk.EventDocumentWritten {
			// React to document changes
		}
		return nil
	}

# Build Requirements

Plugins must be built as WASM reactor modules:

	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .

The -buildmode=c-shared flag is required. It creates a reactor module that
initialises and stays alive, rather than running main() and exiting.

# Plugin Locations

Plugins are loaded from multiple locations in priority order:

 1. Embedded (compiled into llmd binary)
 2. Bundled (same directory as llmd binary)
 3. Global (~/.llmd/plugins/)
 4. Project (.llmd/plugins/)

Later plugins can override commands from earlier plugins. For example, placing
core.wasm in .llmd/plugins/ overrides the embedded core plugin.

# Debugging

If your plugin fails to load, check:

  - Build tags: Ensure //go:build wasip1 is present
  - Build mode: Must use -buildmode=c-shared
  - Registration: Must use init(), not main()
  - Dependencies: All imports must support wasip1
*/
package sdk
