// rm.go implements the "llmd rm" command for soft-deleting documents.
//
// Separated from document.go to isolate deletion logic including recursive
// deletion handling.
//
// Design: Rm performs soft-delete only - documents can be recovered via
// restore until vacuum permanently removes them. The -r flag enables batch
// deletion of entire path hierarchies while maintaining recoverability.

package document

import (
	"fmt"
	"io"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/rm"
	"github.com/spf13/cobra"
)

func (e *Extension) newRmCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rm <path|key>",
		Short: "Delete a document",
		Long:  `Soft-delete a document (recoverable via restore).`,
		Args:  cobra.ExactArgs(1),
		RunE:  e.runRm,
	}
	c.Flags().BoolP(extension.FlagRecursive, "r", false, "Delete all documents under path")
	c.Flags().Int(extension.FlagVersion, 0, "Delete only this specific version")
	return c
}

func (e *Extension) runRm(c *cobra.Command, args []string) error {
	ctx := c.Context()
	recursive, _ := c.Flags().GetBool(extension.FlagRecursive)
	version, _ := c.Flags().GetInt(extension.FlagVersion)
	opts := rm.Options{Recursive: recursive, Version: version}
	p := args[0]

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	result, err := rm.Run(ctx, w, e.svc, p, opts)

	log.Event("document:rm", "delete").
		Author(cmd.Author()).
		Path(p).
		Detail("count", len(result.Deleted)).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("rm %q: %w", p, err))
	}
	return cmd.PrintJSON(result)
}
