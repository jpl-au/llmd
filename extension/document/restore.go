// restore.go implements the "llmd restore" command for recovering deleted documents.
//
// Separated from document.go to isolate recovery logic.
//
// Design: Restore reverses soft-delete, making the document visible again
// with all its version history intact. This provides a safety net against
// accidental rm commands - data isn't lost until vacuum is run.

package document

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

// restoreResult contains the outcome of a restore operation.
type restoreResult struct {
	Path string `json:"path"`
	Key  string `json:"key,omitempty"`
}

func (e *Extension) newRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <path|key>",
		Short: "Restore a deleted document",
		Long:  `Restore a soft-deleted document by path or key.`,
		Args:  cobra.ExactArgs(1),
		RunE:  e.runRestore,
	}
}

func (e *Extension) runRestore(c *cobra.Command, args []string) error {
	ctx := c.Context()
	p := args[0]
	key := ""

	// For 8-char inputs, try path first then key
	if len(p) == 8 {
		// Try path first (need includeDeleted=true for restore)
		_, err := e.svc.Latest(ctx, p, true)
		if err != nil {
			// Path not found, try as key
			doc, keyErr := e.svc.ByKey(ctx, p)
			if keyErr != nil {
				// Neither found, return original error
				return cmd.PrintJSONError(fmt.Errorf("path or key %q: %w", p, err))
			}
			key = p
			p = doc.Path
		}
	}

	err := e.svc.Restore(ctx, p)

	log.Event("document:restore", "restore").
		Author(cmd.Author()).
		Path(p).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("restore %q: %w", p, err))
	}

	if !cmd.JSON() {
		if key != "" {
			fmt.Fprintf(cmd.Out(), "Restored %s (from key %s)\n", p, key)
		} else {
			fmt.Fprintf(cmd.Out(), "Restored %s\n", p)
		}
	}
	return cmd.PrintJSON(restoreResult{Path: p, Key: key})
}
