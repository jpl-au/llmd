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
		Args:  cobra.MaximumNArgs(1),
		RunE:  e.runRm,
	}
	c.Flags().BoolP(extension.FlagRecursive, "r", false, "Delete all documents under path")
	c.Flags().Int(extension.FlagVersion, 0, "Delete only this specific version")
	c.Flags().StringP(extension.FlagKey, "k", "", "Delete by version key (8-char identifier)")
	return c
}

func (e *Extension) runRm(c *cobra.Command, args []string) error {
	ctx := c.Context()
	recursive, _ := c.Flags().GetBool(extension.FlagRecursive)
	version, _ := c.Flags().GetInt(extension.FlagVersion)
	keyFlag, _ := c.Flags().GetString(extension.FlagKey)

	if len(args) == 0 && keyFlag == "" {
		return cmd.PrintJSONError(fmt.Errorf("requires either a path argument or --key flag"))
	}

	if version < 0 {
		return cmd.PrintJSONError(fmt.Errorf("version must be >= 0, got %d", version))
	}

	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	opts := rm.Options{Recursive: recursive, Version: version, Key: keyFlag}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	l := log.Event("document:rm", "delete").
		Author(cmd.Author()).
		Path(path).
		Detail("key", keyFlag).
		Detail("recursive", recursive)

	result, err := rm.Run(ctx, w, e.svc, path, opts)
	if err != nil {
		l.Write(err)
		return cmd.PrintJSONError(fmt.Errorf("rm %q: %w", path, err))
	}

	l.Resolved(result.Path).
		Detail("count", len(result.Deleted)).
		Write(nil)

	return cmd.PrintJSON(result)
}
