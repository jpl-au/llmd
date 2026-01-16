// cat.go implements the "llmd cat" command for reading document contents.
//
// Separated from document.go to isolate output formatting logic including
// line numbering, line range extraction, and terminal rendering with glamour.
//
// Design: Cat behaves like Unix cat with enhancements for versioned documents.
// Terminal output gets glamour markdown rendering; pipe/redirect gets raw
// markdown. The -l flag uses colon syntax (10:20) matching sed/awk conventions.

package document

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/cat"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (e *Extension) newCatCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "cat <path>",
		Short: "Read a document",
		Long:  `Output the contents of a document to stdout.`,
		Args:  cobra.ExactArgs(1),
		RunE:  e.runCat,
	}
	c.Flags().IntP(extension.FlagVersion, "v", 0, "Read specific version")
	c.Flags().BoolP(extension.FlagDeleted, "D", false, "Read a deleted document")
	c.Flags().BoolP(extension.FlagNumber, "n", false, "Number all output lines")
	c.Flags().StringP(extension.FlagLines, "l", "", "Line range (e.g., 10:20, 5:, :15)")
	c.Flags().Bool(extension.FlagRaw, false, "Output raw markdown without rendering")
	return c
}

func (e *Extension) runCat(c *cobra.Command, args []string) error {
	ctx := c.Context()
	ver, _ := c.Flags().GetInt(extension.FlagVersion)
	del, _ := c.Flags().GetBool(extension.FlagDeleted)
	lineNums, _ := c.Flags().GetBool(extension.FlagNumber)
	lineRange, _ := c.Flags().GetString(extension.FlagLines)
	raw, _ := c.Flags().GetBool(extension.FlagRaw)

	opts := cat.Options{
		Version:        ver,
		IncludeDeleted: del,
		LineNumbers:    lineNums,
		MaxLineLength:  e.cfg.MaxLineLength(),
	}

	// Parse line range (e.g., "10:20", "5:", ":15")
	if lineRange != "" {
		start, end, err := parseLineRange(lineRange)
		if err != nil {
			return cmd.PrintJSONError(err)
		}
		opts.StartLine = start
		opts.EndLine = end
	}

	p := args[0]
	var result cat.Result
	var err error

	defer func() {
		b := log.Event("document:cat", "read").Author(cmd.Author()).Path(p)
		if result.Document != nil {
			b = b.Version(result.Document.Version)
		}
		b.Write(err)
	}()

	if cmd.JSON() {
		result, err = cat.Run(ctx, io.Discard, e.svc, p, opts)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("cat %q: %w", p, err))
		}
		return cmd.PrintJSON(result.Document.ToJSON(true))
	}

	// Render with glamour if TTY and not --raw
	if !raw && term.IsTerminal(int(os.Stdout.Fd())) {
		var buf bytes.Buffer
		result, err = cat.Run(ctx, &buf, e.svc, p, opts)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("cat %q: %w", p, err))
		}
		rendered, renderErr := glamour.Render(buf.String(), "dark")
		if renderErr == nil {
			fmt.Fprint(cmd.Out(), rendered)
			return nil
		}
	}

	result, err = cat.Run(ctx, cmd.Out(), e.svc, p, opts)
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("cat %q: %w", p, err))
	}
	return nil
}

// parseLineRange parses a line range string like "10:20", "5:", or ":15".
// Returns start and end line numbers (1-indexed), where 0 means unspecified.
func parseLineRange(s string) (start, end int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid line range %q: expected format START:END", s)
	}

	if parts[0] != "" {
		_, err := fmt.Sscanf(parts[0], "%d", &start)
		if err != nil || start < 1 {
			return 0, 0, fmt.Errorf("invalid start line %q", parts[0])
		}
	}

	if parts[1] != "" {
		_, err := fmt.Sscanf(parts[1], "%d", &end)
		if err != nil || end < 1 {
			return 0, 0, fmt.Errorf("invalid end line %q", parts[1])
		}
	}

	if start > 0 && end > 0 && start > end {
		return 0, 0, fmt.Errorf("start line %d is greater than end line %d", start, end)
	}

	return start, end, nil
}
