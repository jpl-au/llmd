// restore.go implements the "llmd restore" command for recovering deleted documents.
//
// Separated from document.go to isolate recovery logic.
//
// Design: Restore reverses soft-delete, making the document visible again
// with all its version history intact. This provides a safety net against
// accidental rm commands - data isn't lost until vacuum is run.
//
// Supports multiple path arguments for batch restore operations, consistent
// with rm which supports batch deletion. The --key flag is restricted to
// single-path operations to avoid ambiguity.

package document

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

// restoreResult contains the outcome of a restore operation.
type restoreResult struct {
	Path string `json:"path"`
	Key  string `json:"key,omitempty"`
}

func (e *Extension) newRestoreCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "restore <path|key>...",
		Short: "Restore deleted documents",
		Long: `Restore one or more soft-deleted documents by path or key.

Multiple paths can be specified to restore several documents at once.
The --key flag only works with a single path.`,
		Args: cobra.ArbitraryArgs,
		RunE: e.runRestore,
	}
	c.Flags().StringP(extension.FlagKey, "k", "", "Restore by version key (8-char identifier)")
	return c
}

func (e *Extension) runRestore(c *cobra.Command, args []string) error {
	ctx := c.Context()
	keyFlag, _ := c.Flags().GetString(extension.FlagKey)

	if len(args) == 0 && keyFlag == "" {
		return cmd.PrintJSONError(fmt.Errorf("requires either a path argument or --key flag"))
	}

	// --key flag only works with single path
	if len(args) > 1 && keyFlag != "" {
		return cmd.PrintJSONError(fmt.Errorf("--key flag cannot be used with multiple paths"))
	}

	// Single path or --key mode
	if len(args) <= 1 {
		input := ""
		if len(args) > 0 {
			input = args[0]
		}

		var path, key string

		if keyFlag != "" {
			// Explicit key provided - use it directly
			doc, err := e.svc.ByKey(ctx, keyFlag)
			if err != nil {
				return cmd.PrintJSONError(fmt.Errorf("key %q: %w", keyFlag, err))
			}
			path = doc.Path
			key = keyFlag
		} else {
			// Resolve input as path or key (includeDeleted=true for restore)
			doc, isKey, err := e.svc.Resolve(ctx, input, true)
			if err != nil {
				return cmd.PrintJSONError(fmt.Errorf("%q: %w", input, err))
			}
			path = doc.Path
			if isKey {
				key = input
			}
		}

		err := e.svc.Restore(ctx, path)

		log.Event("document:restore", "restore").
			Author(cmd.Author()).
			Path(path).
			Write(err)

		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("restore %q: %w", path, err))
		}

		if !cmd.JSON() {
			if key != "" {
				fmt.Fprintf(cmd.Out(), "Restored %s (from key %s)\n", path, key)
			} else {
				fmt.Fprintf(cmd.Out(), "Restored %s\n", path)
			}
		}
		return cmd.PrintJSON(restoreResult{Path: path, Key: key})
	}

	// Multiple paths mode
	var results []restoreResult
	l := log.Event("document:restore", "restore").
		Author(cmd.Author()).
		Detail("paths", args)
	defer func() { l.Detail("count", len(results)).Write(nil) }()

	for _, input := range args {
		// Resolve input as path or key
		doc, isKey, err := e.svc.Resolve(ctx, input, true)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("%q: %w", input, err))
		}

		if err := e.svc.Restore(ctx, doc.Path); err != nil {
			return cmd.PrintJSONError(fmt.Errorf("restore %q: %w", doc.Path, err))
		}

		result := restoreResult{Path: doc.Path}
		if isKey {
			result.Key = input
		}
		results = append(results, result)

		if !cmd.JSON() {
			if isKey {
				fmt.Fprintf(cmd.Out(), "Restored %s (from key %s)\n", doc.Path, input)
			} else {
				fmt.Fprintf(cmd.Out(), "Restored %s\n", doc.Path)
			}
		}
	}

	return cmd.PrintJSON(results)
}
