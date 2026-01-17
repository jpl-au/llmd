// guide.go implements the "llmd guide" command for documentation access.
//
// Separated from extension.go to isolate documentation rendering logic
// including terminal detection and glamour markdown formatting.
//
// Design: Guides are embedded in the binary via the guide package, ensuring
// documentation is always available without external files. Terminal output
// gets glamour rendering for readability; pipe/redirect gets raw markdown
// for machine consumption and LLM context loading.

package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/guide"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide [command]",
		Short: "Show the llmd usage guide",
		Long: `Outputs the llmd guide for LLMs and humans.

  llmd guide           # main guide
  llmd guide init      # detailed init guide
  llmd guide write     # detailed write guide`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			content, err := guide.Get(name)
			if err != nil {
				available, listErr := guide.List()
				if listErr != nil {
					return listErr
				}
				return cmd.PrintJSONError(fmt.Errorf("guide %q not found. Available: %s", name, strings.Join(available, ", ")))
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
