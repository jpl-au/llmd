// glob.go implements the "llmd glob" command for path pattern matching.
//
// Separated from search.go because glob operates purely on paths (no content),
// making it fundamentally different from find (FTS5) and grep (regex on content).
// Glob queries the database directly without loading document content.

package search

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

func (e *Extension) newGlobCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "glob [pattern]",
		Short: "List document paths matching a pattern",
		Long: `List document paths matching a glob pattern.

Queries the database and returns matching document paths.
Use with 'llmd cat' to read document contents.

Supports glob patterns: *, **, ?

Examples:
  llmd glob              # All documents
  llmd glob "docs/*"     # Direct children of docs/
  llmd glob "docs/**"    # All under docs/
  llmd glob -j           # JSON output`,
		Args: cobra.MaximumNArgs(1),
		RunE: e.runGlob,
	}
}

func (e *Extension) runGlob(c *cobra.Command, args []string) error {
	ctx := c.Context()
	pattern := ""
	if len(args) > 0 {
		pattern = args[0]
	}

	l := log.Event("search:glob", "list").
		Author(cmd.Author()).
		Detail("pattern", pattern)

	paths, err := e.svc.Glob(ctx, pattern)
	if err != nil {
		l.Write(err)
		return cmd.PrintJSONError(fmt.Errorf("glob %q: %w", pattern, err))
	}

	l.Detail("count", len(paths)).Write(nil)

	if cmd.JSON() {
		return cmd.PrintJSON(paths)
	}

	for _, p := range paths {
		fmt.Fprintln(cmd.Out(), p)
	}
	return nil
}
