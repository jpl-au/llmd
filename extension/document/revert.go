// revert.go implements the "llmd revert" command for version rollback.
//
// Separated from document.go to isolate version management logic including
// key-based lookup for direct version reference.
//
// Design: Revert is forward-moving - it creates a new version with old content
// rather than deleting newer versions. This preserves complete history and
// enables audit trails. The 8-char key system allows direct version references
// without needing to know the document path.

package document

import (
	"fmt"
	"io"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/revert"
	"github.com/spf13/cobra"
)

func (e *Extension) newRevertCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "revert <path> [version]",
		Short: "Revert a document to a previous version",
		Long: `Revert a document to a previous version by creating a new version with the old content.

This is a forward-moving operation - it preserves history by creating a new version
rather than deleting versions.

The target can be specified as:
  - A path and version number: llmd revert docs/api 3
  - A key (8-char identifier): llmd revert --key abc12345`,
		Args: cobra.MaximumNArgs(2),
		RunE: e.runRevert,
	}
	c.Flags().StringP(extension.FlagKey, "k", "", "Revert to version by key (8-char identifier)")
	return c
}

func (e *Extension) runRevert(c *cobra.Command, args []string) error {
	ctx := c.Context()
	keyFlag, _ := c.Flags().GetString(extension.FlagKey)

	if len(args) == 0 && keyFlag == "" {
		return cmd.PrintJSONError(fmt.Errorf("requires either a path argument or --key flag"))
	}

	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	version := 0

	if len(args) == 2 {
		_, err := fmt.Sscanf(args[1], "%d", &version)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("invalid version %q: must be a number", args[1]))
		}
		if version < 1 {
			return cmd.PrintJSONError(fmt.Errorf("version must be >= 1, got %d", version))
		}
	}

	opts := revert.Options{
		Author:  cmd.Author(),
		Message: cmd.Message(),
		Key:     keyFlag,
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	l := log.Event("document:revert", "revert").
		Author(cmd.Author()).
		Path(target).
		Version(version).
		Detail("key", keyFlag)

	result, err := revert.Run(ctx, w, e.svc, target, version, opts)
	if err != nil {
		l.Write(err)
		return cmd.PrintJSONError(err)
	}

	l.Resolved(result.Path).
		ResultVersion(result.NewVersion).
		Write(nil)

	return cmd.PrintJSON(result)
}
