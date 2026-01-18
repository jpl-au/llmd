// mv.go implements the "llmd mv" command for renaming/moving documents.
//
// Separated from document.go to isolate move logic.
//
// Design: Mv updates the path in-place rather than creating a copy, preserving
// version history under the new path. This matches Unix mv semantics:
//
//   - Single source: `mv source dest` renames source to dest
//   - Multiple sources: `mv source1 source2 ... prefix/` moves all sources under prefix
//
// The trailing slash on destination signals "move into" rather than "rename to",
// consistent with how Unix mv interprets directory destinations. References in
// tags and links are automatically updated to maintain consistency.

package document

import (
	"fmt"
	"path"
	"strings"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

// mvResult contains the outcome of a single move operation.
type mvResult struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (e *Extension) newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <source>... <dest>",
		Short: "Move/rename documents",
		Long: `Rename a document or move multiple documents to a new prefix.

Single source: mv source dest
Multiple sources: mv source1 source2 ... dest/

When destination ends with /, sources are moved under that prefix
preserving their base names (e.g., docs/a -> archive/a).`,
		Args: cobra.MinimumNArgs(2),
		RunE: e.runMv,
	}
}

func (e *Extension) runMv(c *cobra.Command, args []string) error {
	ctx := c.Context()

	// Last argument is destination, rest are sources
	sources := args[:len(args)-1]
	dest := args[len(args)-1]

	if dest == "" {
		return cmd.PrintJSONError(fmt.Errorf("destination path cannot be empty"))
	}

	// Determine if this is a "move into prefix" operation:
	// - Multiple sources always require prefix mode
	// - Trailing slash signals prefix mode even with single source
	prefixMode := len(sources) > 1 || strings.HasSuffix(dest, "/")
	destPrefix := strings.TrimSuffix(dest, "/")

	var results []mvResult
	l := log.Event("document:mv", "move").Author(cmd.Author())
	if len(sources) == 1 {
		l.Path(sources[0])
	} else {
		l.Detail("sources", sources)
	}
	l.Detail("dest", dest)
	defer func() { l.Detail("count", len(results)).Write(nil) }()

	for _, src := range sources {
		if src == "" {
			return cmd.PrintJSONError(fmt.Errorf("source path cannot be empty"))
		}

		var target string
		if prefixMode {
			// Move into prefix: docs/readme -> archive/readme
			base := path.Base(src)
			target = path.Join(destPrefix, base)
		} else {
			// Direct rename: source -> dest
			target = dest
		}

		err := e.svc.Move(ctx, src, target)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("mv %q to %q: %w", src, target, err))
		}

		results = append(results, mvResult{From: src, To: target})

		if !cmd.JSON() {
			fmt.Fprintf(cmd.Out(), "Moved %s -> %s\n", src, target)
		}
	}

	// Return single object for single move, array for multiple
	if len(results) == 1 {
		return cmd.PrintJSON(results[0])
	}
	return cmd.PrintJSON(results)
}
