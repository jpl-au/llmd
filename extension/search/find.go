// find.go implements the "llmd find" command for FTS5 full-text search.
//
// Separated from search.go to isolate FTS5-specific logic. Full-text search
// uses SQLite's FTS5 extension which has different query semantics than regex
// (grep) or glob matching - keeping them separate prevents confusion.

package search

import (
	"fmt"
	"io"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/find"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/spf13/cobra"
)

func (e *Extension) newFindCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "find <query>",
		Short: "Full-text search across documents",
		Long: `Full-text search across documents.

Supports FTS5 query syntax including prefix matching with *.`,
		Args: cobra.ExactArgs(1),
		RunE: e.runFind,
	}
	c.Flags().StringP(extension.FlagPath, "p", "", "Scope search to path prefix")
	c.Flags().BoolP(extension.FlagPathsOnly, "l", false, "Only output paths")
	c.Flags().BoolP(extension.FlagDeleted, "D", false, "Search deleted documents only")
	c.Flags().BoolP(extension.FlagAll, "A", false, "Search all documents (including deleted)")
	return c
}

func (e *Extension) runFind(c *cobra.Command, args []string) error {
	ctx := c.Context()
	query := args[0]
	prefix, _ := c.Flags().GetString(extension.FlagPath)
	del, _ := c.Flags().GetBool(extension.FlagDeleted)
	all, _ := c.Flags().GetBool(extension.FlagAll)
	pathsOnly, _ := c.Flags().GetBool(extension.FlagPathsOnly)

	opts := find.Options{
		Prefix:      prefix,
		IncludeAll:  all,
		DeletedOnly: del,
		PathsOnly:   pathsOnly,
	}

	var result find.Result
	var err error

	if cmd.JSON() {
		result, err = find.Run(ctx, io.Discard, e.svc, query, opts)
	} else {
		result, err = find.Run(ctx, cmd.Out(), e.svc, query, opts)
	}

	log.Event("search:find", "search").
		Author(cmd.Author()).
		Path(prefix).
		Detail("query", query).
		Detail("count", len(result.Documents)).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("find %q: %w", query, err))
	}

	if cmd.JSON() {
		items := make([]store.DocJSON, len(result.Documents))
		for i := range result.Documents {
			items[i] = result.Documents[i].ToJSON(!pathsOnly)
		}
		return cmd.PrintJSON(items)
	}
	return nil
}
