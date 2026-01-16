// write.go implements the "llmd write" command for creating/updating documents.
//
// Separated from document.go to isolate input handling (stdin, argument, file).
//
// Design: Write accepts content from multiple sources in priority order:
// 1. Direct argument (for short content)
// 2. File flag (for existing files)
// 3. Stdin (for piping)
// This flexibility supports both interactive and scripted workflows.

package document

import (
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

// writeResult contains the outcome of a write operation.
type writeResult struct {
	Path string `json:"path"`
}

func (e *Extension) newWriteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "write <path> [content]",
		Short: "Write a document",
		Long:  `Create or update a document. Content from argument, stdin, or -f flag.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  e.runWrite,
	}
	c.Flags().StringP(extension.FlagFile, "f", "", "Read content from file")
	return c
}

func (e *Extension) runWrite(c *cobra.Command, args []string) error {
	ctx := c.Context()
	p := args[0]
	var content string

	file, _ := c.Flags().GetString(extension.FlagFile)
	switch {
	case len(args) >= 2:
		content = args[1]
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("read file %q: %w", file, err))
		}
		content = string(data)
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("read stdin: %w", err))
		}
		content = string(data)
	}

	err := e.svc.Write(ctx, p, content, cmd.Author(), cmd.Message())

	log.Event("document:write", "write").
		Author(cmd.Author()).
		Path(p).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("write %q: %w", p, err))
	}

	if !cmd.JSON() {
		fmt.Fprintf(cmd.Out(), "Wrote %s\n", p)
	}
	return cmd.PrintJSON(writeResult{Path: p})
}
