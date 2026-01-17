// diff.go implements the "llmd diff" command for comparing document versions.
//
// Separated from document.go to isolate diff logic including version range
// parsing and multi-mode comparison (versions, documents, filesystem files).
//
// Design: Diff supports multiple comparison modes for flexibility:
// - Version to version (within same document)
// - Document to document (different paths)
// - Filesystem file to document (for sync verification)
// Output uses unified diff format compatible with patch tools.

package document

import (
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/diff"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

func (e *Extension) newDiffCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "diff <path|key> [doc-path]",
		Short: "Show differences between document versions",
		Long: `Show differences between document versions or two documents.

Examples:
  llmd diff docs/readme              # Compare latest with previous version
  llmd diff docs/readme -v 3:5       # Compare version 3 with version 5
  llmd diff docs/readme docs/other   # Compare two different documents
  llmd diff -f ./local.md docs/readme    # Compare filesystem file with stored document`,
		Args: cobra.RangeArgs(1, 2),
		RunE: e.runDiff,
	}
	c.Flags().StringP(extension.FlagVersions, "v", "", "Version range (e.g., 3:5)")
	c.Flags().BoolP(extension.FlagDeleted, "D", false, "Allow diffing deleted documents")
	c.Flags().BoolP(extension.FlagFile, "f", false, "Treat first path as filesystem file")
	c.Flags().Bool(extension.FlagRaw, false, "Output without colour")
	return c
}

func (e *Extension) runDiff(c *cobra.Command, args []string) error {
	verRange, _ := c.Flags().GetString(extension.FlagVersions)
	del, _ := c.Flags().GetBool(extension.FlagDeleted)
	isFile, _ := c.Flags().GetBool(extension.FlagFile)
	raw, _ := c.Flags().GetBool(extension.FlagRaw)
	path := args[0]

	var opts diff.Options
	var err error
	opts.IncludeDeleted = del

	if verRange != "" {
		opts.Version1, opts.Version2, err = diff.ParseVersionRange(verRange)
		if err != nil {
			return cmd.PrintJSONError(err)
		}
	}

	if len(args) == 2 {
		opts.Path2 = args[1]
	}

	// Read file content if -f flag is set
	if isFile {
		if opts.Path2 == "" {
			return cmd.PrintJSONError(fmt.Errorf("-f/--file requires two arguments: llmd diff -f <file> <doc-path>"))
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("reading file %s: %w", path, err))
		}
		opts.FileContent = string(b)
	}

	ctx := c.Context()

	// Resolve path or key to get actual document path for logging
	// (only when not using -f flag, where path is a filesystem file)
	logPath := path
	if !isFile {
		if doc, _, resolveErr := e.svc.Resolve(ctx, path, del); resolveErr == nil {
			logPath = doc.Path
		}
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}
	opts.Colour = !raw

	r, err := diff.Run(ctx, w, e.svc, path, opts)

	log.Event("document:diff", "diff").
		Author(cmd.Author()).
		Path(logPath).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("diff %q: %w", path, err))
	}

	return cmd.PrintJSON(map[string]string{
		"old":  r.Old,
		"new":  r.New,
		"diff": r.Format(false),
	})
}
