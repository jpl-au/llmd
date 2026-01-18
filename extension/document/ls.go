// ls.go implements the "llmd ls" command for listing documents.
//
// Separated from document.go to isolate listing and tree-formatting logic.
//
// Design: Ls mimics Unix ls with document-specific extensions. The -t flag
// shows a tree view (unique to llmd), -l shows metadata like versions and
// timestamps. Tag filtering enables workflow-specific document views.

package document

import (
	"fmt"
	"io"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/ls"
	"github.com/spf13/cobra"
)

func (e *Extension) newLsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ls [prefix]",
		Short: "List documents",
		Long:  `List all documents, optionally filtered by path prefix.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  e.runLs,
	}
	c.Flags().BoolP(extension.FlagAll, "A", false, "Include deleted documents")
	c.Flags().BoolP(extension.FlagDeleted, "D", false, "Show only deleted documents")
	c.Flags().BoolP(extension.FlagTree, "t", false, "Display as tree")
	c.Flags().BoolP(extension.FlagLong, "l", false, "Long format with metadata")
	c.Flags().String(extension.FlagTag, "", "Filter by tag")
	c.Flags().StringP(extension.FlagSort, "s", "", "Sort by: name, time")
	c.Flags().BoolP(extension.FlagReverse, "R", false, "Reverse sort order")
	return c
}

func (e *Extension) runLs(c *cobra.Command, args []string) error {
	ctx := c.Context()
	opts := ls.Options{}
	if len(args) > 0 {
		opts.Prefix = args[0]
	}
	opts.IncludeAll, _ = c.Flags().GetBool(extension.FlagAll)
	opts.DeletedOnly, _ = c.Flags().GetBool(extension.FlagDeleted)
	opts.Tree, _ = c.Flags().GetBool(extension.FlagTree)
	opts.Long, _ = c.Flags().GetBool(extension.FlagLong)
	opts.Tag, _ = c.Flags().GetString(extension.FlagTag)
	opts.Reverse, _ = c.Flags().GetBool(extension.FlagReverse)

	sortBy, _ := c.Flags().GetString(extension.FlagSort)
	if sortBy != "" && sortBy != "name" && sortBy != "time" {
		return cmd.PrintJSONError(fmt.Errorf("invalid sort field %q: must be 'name' or 'time'", sortBy))
	}
	opts.Sort = ls.SortField(sortBy)

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	result, err := ls.Run(ctx, w, e.svc, opts)

	log.Event("document:ls", "list").
		Author(cmd.Author()).
		Path(opts.Prefix).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("ls %q: %w", opts.Prefix, err))
	}
	return cmd.PrintJSON(result.ToJSON())
}
