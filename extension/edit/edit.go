// Package edit provides the edit extension for llmd.
// It registers commands: edit, sed.
package edit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/edit"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/sed"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the edit extension.
type Extension struct {
	svc service.Service
}

// Compile-time interface compliance. Catches missing methods at build time
// rather than runtime, making interface changes safer to refactor.
var (
	_ extension.Extension     = (*Extension)(nil)
	_ extension.Initializable = (*Extension)(nil)
)

// Name returns "edit" - this extension provides text editing commands.
func (e *Extension) Name() string { return "edit" }

// Init receives the shared service from the extension context.
func (e *Extension) Init(ctx extension.Context) error {
	e.svc = ctx.Service()
	return nil
}

// Commands returns edit and sed commands for document modification.
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		e.newEditCmd(),
		e.newSedCmd(),
	}
}

// MCPTools returns nil - MCP editing tools are in internal/mcp.
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}

// --- edit command ---

func (e *Extension) newEditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit <path|key> [old] [new]",
		Short: "Partial edit via search/replace or line range",
		Long: `Edit a document by replacing text or lines.

Search/replace mode (positional or flags):
  llmd edit docs/readme "old text" "new text"
  llmd edit docs/readme -o "old text" -n "new text"
  llmd edit docs/readme -i "OLD TEXT" "new text"  # case-insensitive

Line range mode (replaces lines with stdin):
  llmd edit docs/readme -l 5:10 <<< "replacement content"`,
		Args: cobra.RangeArgs(1, 3),
		RunE: e.runEdit,
	}
	c.Flags().String(extension.FlagOld, "", "Text to find")
	c.Flags().String(extension.FlagNew, "", "Text to replace with")
	c.Flags().StringP(extension.FlagLines, "l", "", "Line range (e.g., 5:10)")
	c.Flags().BoolP(extension.FlagIgnoreCase, "i", false, "Case-insensitive matching")
	return c
}

func (e *Extension) runEdit(c *cobra.Command, args []string) error {
	ctx := c.Context()
	lineRange, _ := c.Flags().GetString(extension.FlagLines)
	path := args[0]

	var result edit.Result
	var err error
	if lineRange != "" {
		result, err = e.runEditLineRange(ctx, path, lineRange)
	} else {
		result, err = e.runEditReplace(ctx, c, args)
	}

	log.Event("edit:edit", "edit").
		Author(cmd.Author()).
		Path(path).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("edit %q: %w", path, err))
	}
	return cmd.PrintJSON(result)
}

func (e *Extension) runEditLineRange(ctx context.Context, path, lineRange string) (edit.Result, error) {
	start, end, err := edit.ParseLineRange(lineRange)
	if err != nil {
		return edit.Result{}, fmt.Errorf("parse line range %q: %w", lineRange, err)
	}

	replacement, err := io.ReadAll(os.Stdin)
	if err != nil {
		return edit.Result{}, fmt.Errorf("read stdin: %w", err)
	}

	opts := edit.LineRangeOptions{
		Start:   start,
		End:     end,
		Author:  cmd.Author(),
		Message: cmd.Message(),
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	return edit.RunLineRange(ctx, w, e.svc, path, string(replacement), opts)
}

func (e *Extension) runEditReplace(ctx context.Context, c *cobra.Command, args []string) (edit.Result, error) {
	old, _ := c.Flags().GetString(extension.FlagOld)
	newStr, _ := c.Flags().GetString(extension.FlagNew)
	ignoreCase, _ := c.Flags().GetBool(extension.FlagIgnoreCase)
	if len(args) >= 3 {
		old, newStr = args[1], args[2]
	}
	if old == "" {
		return edit.Result{}, errors.New("old text is required (use positional args or --old flag)")
	}

	opts := edit.Options{
		Old:             old,
		New:             newStr,
		CaseInsensitive: ignoreCase,
		Author:          cmd.Author(),
		Message:         cmd.Message(),
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	return edit.Run(ctx, w, e.svc, args[0], opts)
}

// --- sed command ---

func (e *Extension) newSedCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sed [-i] <expression> <path|key>",
		Short: "Stream editor for documents",
		Long: `Edit documents using sed-style substitution syntax.

  llmd sed -i 's/old/new/' docs/readme
  llmd sed -i 's/old/new/g' docs/readme   # replace all occurrences
  llmd sed -i 's|old|new|' docs/readme    # alternate delimiter

The -i flag (in-place) is required, matching sed behaviour.
Supports the 'g' flag for global replacement.
Only substitution (s) commands are supported.`,
		Args: cobra.ExactArgs(2),
		RunE: e.runSed,
	}
	c.Flags().BoolP(extension.FlagInPlace, "i", false, "Edit file in place (required)")
	return c
}

func (e *Extension) runSed(c *cobra.Command, args []string) error {
	ctx := c.Context()
	inPlace, _ := c.Flags().GetBool(extension.FlagInPlace)
	if !inPlace {
		return cmd.PrintJSONError(errors.New("the -i flag is required (sed only supports in-place editing)"))
	}

	expr, path := args[0], args[1]

	opts := sed.Options{
		Author:  cmd.Author(),
		Message: cmd.Message(),
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	result, err := sed.Run(ctx, w, e.svc, path, expr, opts)

	log.Event("edit:sed", "edit").
		Author(cmd.Author()).
		Path(path).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("sed %q: %w", path, err))
	}
	return cmd.PrintJSON(result)
}
