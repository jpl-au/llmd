// mv.go implements the "llmd mv" command for renaming/moving documents.
//
// Separated from document.go to isolate move logic.
//
// Design: Mv updates the path in-place rather than creating a copy, preserving
// version history under the new path. This matches Unix mv semantics. References
// in tags and links are automatically updated to maintain consistency.

package document

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

// mvResult contains the outcome of a move operation.
type mvResult struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (e *Extension) newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <source> <dest>",
		Short: "Move/rename a document",
		Long:  `Rename a document or move it to a new path.`,
		Args:  cobra.ExactArgs(2),
		RunE:  e.runMv,
	}
}

func (e *Extension) runMv(c *cobra.Command, args []string) error {
	ctx := c.Context()
	src, dst := args[0], args[1]

	err := e.svc.Move(ctx, src, dst)

	log.Event("document:mv", "move").
		Author(cmd.Author()).
		Path(src).
		Detail("from", src).
		Detail("to", dst).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("mv %q to %q: %w", src, dst, err))
	}

	if !cmd.JSON() {
		fmt.Fprintf(cmd.Out(), "Moved %s -> %s\n", src, dst)
	}
	return cmd.PrintJSON(mvResult{From: src, To: dst})
}
