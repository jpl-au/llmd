// history.go implements the "llmd history" command for viewing version history.
//
// Separated from document.go to isolate history display formatting including
// terminal width detection for tabular output.
//
// Design: History shows all versions with metadata (author, timestamp, message).
// This enables audit trails and informed decisions about which version to
// revert to. The -D flag includes deleted versions for forensic analysis.

package document

import (
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/history"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (e *Extension) newHistoryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "history <path|key>",
		Short: "Show document history",
		Long:  `Display version history for a document.`,
		Args:  cobra.ExactArgs(1),
		RunE:  e.runHistory,
	}
	c.Flags().IntP(extension.FlagLimit, "n", 0, "Limit number of versions shown")
	c.Flags().BoolP(extension.FlagDeleted, "D", false, "Include deleted versions")
	c.Flags().BoolP(extension.FlagDiff, "d", false, "Show diffs between versions")
	return c
}

func (e *Extension) runHistory(c *cobra.Command, args []string) error {
	ctx := c.Context()
	limit, _ := c.Flags().GetInt(extension.FlagLimit)
	del, _ := c.Flags().GetBool(extension.FlagDeleted)
	showDiff, _ := c.Flags().GetBool(extension.FlagDiff)
	path := args[0]

	if limit < 0 {
		return cmd.PrintJSONError(fmt.Errorf("limit must be >= 0, got %d", limit))
	}

	opts := history.Options{
		Limit:          limit,
		IncludeDeleted: del,
		ShowDiff:       showDiff,
		Colour:         term.IsTerminal(int(os.Stdout.Fd())),
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	result, err := history.Run(ctx, w, e.svc, path, opts)

	logPath := path
	if len(result.Versions) > 0 {
		logPath = result.Versions[0].Path
	}
	log.Event("document:history", "history").
		Author(cmd.Author()).
		Path(logPath).
		Detail("count", len(result.Versions)).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("history %q: %w", path, err))
	}

	if cmd.JSON() {
		out := make([]store.DocJSON, len(result.Versions))
		for i := range result.Versions {
			out[i] = result.Versions[i].ToJSON(false)
		}
		return cmd.PrintJSON(out)
	}
	return nil
}
