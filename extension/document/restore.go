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
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

// restoreResult contains the outcome of a restore operation.
type restoreResult struct {
	Path string `json:"path"`
	Key  string `json:"key,omitempty"`
}

func (e *Extension) newRestoreCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "restore <path>",
		Short: "Restore a deleted document",
		Long:  `Restore a soft-deleted document by path or key.`,
		Args:  cobra.ExactArgs(1),
		RunE:  e.runRestore,
	}
	c.Flags().StringP(extension.FlagKey, "k", "", "Restore by version key (8-char identifier)")
	return c
}

func (e *Extension) runRestore(c *cobra.Command, args []string) error {
	ctx := c.Context()
	input := args[0]
	keyFlag, _ := c.Flags().GetString(extension.FlagKey)

	var p, key string

	if keyFlag != "" {
		// Explicit key provided - use it directly
		doc, err := e.svc.ByKey(ctx, keyFlag)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("key %q: %w", keyFlag, err))
		}
		p = doc.Path
		key = keyFlag
	} else {
		// Resolve input as path or key (includeDeleted=true for restore)
		doc, isKey, err := e.svc.Resolve(ctx, input, true)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("%q: %w", input, err))
		}
		p = doc.Path
		if isKey {
			key = input
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
