// Package main is a minimal test plugin for verifying the plugin system.
package main

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

func main() {
	sdk.Register(&TestPlugin{})
}

// TestPlugin is a minimal plugin for testing.
type TestPlugin struct{}

// Manifest returns the test plugin metadata.
func (p *TestPlugin) Manifest() sdk.Manifest {
	return sdk.Manifest{
		Name:        "test",
		Version:     "0.1.0",
		Author:      "llmd",
		Description: "Minimal test plugin for verifying plugin system",
		Commands: []sdk.Command{
			{
				Name:        "hello",
				Description: "Say hello",
				MCPEnabled:  true,
				Flags: []sdk.Flag{
					{Name: "name", Short: "n", Type: "string", Default: "world", Description: "Name to greet"},
				},
			},
			{
				Name:        "echo",
				Description: "Echo arguments back",
				MCPEnabled:  true,
			},
		},
	}
}

// ExecuteCommand executes a test command.
func (p *TestPlugin) ExecuteCommand(ctx sdk.Context, cmd string, args []string, flags map[string]any) (sdk.Result, error) {
	switch cmd {
	case "hello":
		name := "world"
		if n, ok := flags["name"].(string); ok && n != "" {
			name = n
		}
		return sdk.TextResult(fmt.Sprintf("Hello, %s!", name)), nil

	case "echo":
		if len(args) == 0 {
			return sdk.TextResult("(no arguments)"), nil
		}
		return sdk.TextResult(fmt.Sprintf("Echo: %v", args)), nil

	default:
		return nil, sdk.ErrUnknownCommand{Command: cmd}
	}
}
