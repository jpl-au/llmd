//go:build wasip1

// This file implements the write command for creating/updating documents.
package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Write defines the write command for creating or updating documents.
//
// The write command reads content from stdin and writes it to the specified
// path. This creates a new version of the document, preserving history.
var Write = sdk.Command{
	Name:        "write",
	Description: "Write a document",
	Usage:       "write <path>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "message", Short: "m", Type: "string", Description: "Version message"},
	},
}

// ExecWrite executes the write command.
//
// Reads content from stdin and writes it to the document at the specified
// path. The author from the execution context is recorded in version history.
// An optional message flag can describe the change.
func ExecWrite(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("write: missing path argument")
	}

	path := args[0]

	message, _ := flags["message"].(string)

	// Read content from stdin
	var content strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("write: reading stdin: %w", err)
	}

	if err := sdk.Host.Write(path, []byte(content.String()), ctx.Author, message); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Wrote %s", path)), nil
}
