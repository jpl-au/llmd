// llm.go implements the "llmd llm" command for LLM integration hints.
//
// Separated from extension.go to isolate LLM-specific documentation that
// helps AI assistants discover available commands and usage patterns.
//
// Design: This is a simple static output command - it prints command hints
// designed for LLM context windows. Unlike "guide" which provides full
// documentation, "llm" provides concise discoverability hints that fit
// in system prompts or tool descriptions.

package core

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/spf13/cobra"
)

func newLlmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "llm",
		Short: "Show LLM documentation hints",
		Long:  `Outputs documentation hints for LLM integration and discoverability.`,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.Out(), `Commands work like standard filesystem/unix tools:
  ls, cat, rm, mv, grep, find, sed, diff, history

Additional commands:
  write     Write stdin to document
  edit      Search/replace or line-range edit
  tag       Manage document tags
  glob      List paths matching pattern
  restore   Restore deleted document
  import    Import from filesystem
  export    Export to filesystem
  sync      Sync filesystem changes to store
  serve     Start MCP server

Use 'llmd guide' for full documentation.
Use 'llmd guide <command>' for command-specific help.`)
		},
	}
}
