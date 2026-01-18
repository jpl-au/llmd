// llm.go implements the "llmd llm" command for LLM integration hints.
//
// Separated from extension.go to isolate LLM-specific documentation that
// helps AI assistants discover available commands and usage patterns.
//
// Design: Reads from guide/llm.md to avoid duplicating content. The guide
// file is the single source of truth for LLM onboarding documentation.

package core

import (
	"fmt"
	"os"

	"github.com/charmbracelet/glamour"
	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/guide"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newLlmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "llm",
		Short: "Getting started guide for LLMs",
		Long:  `Quick reference for LLMs to discover available commands and usage patterns.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			content, err := guide.Get("llm")
			if err != nil {
				return cmd.PrintJSONError(err)
			}

			if term.IsTerminal(int(os.Stdout.Fd())) {
				rendered, err := glamour.Render(content, "dark")
				if err == nil {
					fmt.Fprint(cmd.Out(), rendered)
					return nil
				}
			}

			fmt.Fprint(cmd.Out(), content)
			return nil
		},
	}
}
