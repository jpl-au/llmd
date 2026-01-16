// grep.go implements the "llmd grep" command for regex content searching.
//
// Separated from search.go to isolate regex-specific logic. Unlike FTS5 (find),
// grep uses Go's regexp package for precise pattern matching with familiar
// Unix grep semantics (-i, -v, -l, -c flags). This enables exact matches and
// complex patterns that FTS5's tokenised search cannot provide.

package search

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/grep"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/spf13/cobra"
)

func (e *Extension) newGrepCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "grep <pattern> [path]",
		Short: "Search documents using regex",
		Long: `Search documents using regular expressions, like Unix grep.

  llmd grep "TODO"              # search all documents
  llmd grep "error|warn" docs/  # search with alternation
  llmd grep -i "auth.*token"    # case-insensitive regex
  llmd grep -l "func.*\("       # list matching paths only

For full-text search (FTS5), use 'llmd find' instead.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: e.runGrep,
	}
	c.Flags().BoolP(extension.FlagFilesWithMatch, "l", false, "Only output paths of matching files")
	c.Flags().BoolP(extension.FlagIgnoreCase, "i", false, "Ignore case distinctions")
	c.Flags().BoolP(extension.FlagInvertMatch, "v", false, "Select non-matching lines")
	c.Flags().BoolP(extension.FlagCount, "c", false, "Only print count of matches per document")
	c.Flags().IntP(extension.FlagContext, "C", 0, "Print N lines of context around matches")
	c.Flags().BoolP(extension.FlagRecursive, "r", false, "Search recursively (always enabled, accepted for compatibility)")
	c.Flags().BoolP(extension.FlagDeleted, "D", false, "Search deleted documents only")
	c.Flags().BoolP(extension.FlagAll, "A", false, "Search all documents (including deleted)")
	return c
}

func (e *Extension) runGrep(c *cobra.Command, args []string) error {
	ctx := c.Context()
	pattern := args[0]
	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	del, _ := c.Flags().GetBool(extension.FlagDeleted)
	all, _ := c.Flags().GetBool(extension.FlagAll)
	pathsOnly, _ := c.Flags().GetBool(extension.FlagFilesWithMatch)
	ignoreCase, _ := c.Flags().GetBool(extension.FlagIgnoreCase)
	invert, _ := c.Flags().GetBool(extension.FlagInvertMatch)
	countOnly, _ := c.Flags().GetBool(extension.FlagCount)
	context, _ := c.Flags().GetInt(extension.FlagContext)

	if context < 0 {
		return cmd.PrintJSONError(fmt.Errorf("context lines (-C) must be >= 0, got %d", context))
	}

	opts := grep.Options{
		Path:          path,
		IncludeAll:    all,
		DeletedOnly:   del,
		PathsOnly:     pathsOnly,
		IgnoreCase:    ignoreCase,
		Invert:        invert,
		CountOnly:     countOnly,
		Context:       context,
		MaxLineLength: e.cfg.MaxLineLength(),
	}

	result, err := grep.Run(ctx, cmd.Out(), e.svc, pattern, opts)

	log.Event("search:grep", "search").
		Author(cmd.Author()).
		Path(path).
		Detail("pattern", pattern).
		Detail("count", len(result.Documents)).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("grep %q: %w", pattern, err))
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
