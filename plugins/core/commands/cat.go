//go:build wasip1

// Package commands implements the core plugin's command handlers.
//
// Each command is defined in a separate file with its command definition
// (exported as a variable) and its execution function. This separation
// allows the main plugin file to reference commands without importing
// their full implementation.
package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Cat defines the cat command for reading documents.
//
// The cat command displays the content of a document, similar to the Unix
// cat command. It supports reading specific versions and line numbering.
var Cat = sdk.Command{
	Name:        "cat",
	Description: "Read a document",
	Usage:       "cat [options] <path>...",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "version", Type: "int", Description: "Read specific version"},
		{Name: "n", Type: "bool", Description: "Number output lines"},
	},
}

// ExecCat executes the cat command.
//
// Reads documents from the store and returns their content.
// Supports multiple paths, specific versions, and line numbering.
func ExecCat(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	// Parse args
	var paths []string
	var version int
	var numberLines bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" {
			numberLines = true
		} else if arg == "--version" && i+1 < len(args) {
			i++
			version, _ = strconv.Atoi(args[i])
		} else if strings.HasPrefix(arg, "--version=") {
			version, _ = strconv.Atoi(strings.TrimPrefix(arg, "--version="))
		} else if !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("cat: missing path argument")
	}

	var results []string
	for _, path := range paths {
		content, err := sdk.Host.Read(path, version)
		if err != nil {
			return nil, fmt.Errorf("cat: %s: %w", path, err)
		}

		text := string(content)
		if numberLines {
			text = addLineNumbers(text)
		}
		results = append(results, text)
	}

	output := strings.Join(results, "\n")
	return sdk.RichResult{
		Text: output,
		Data: output,
	}, nil
}

// addLineNumbers prefixes each line with its line number.
func addLineNumbers(s string) string {
	lines := strings.Split(s, "\n")
	width := len(strconv.Itoa(len(lines)))
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d  %s", width, i+1, line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
