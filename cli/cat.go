package cli

// cat reads one or more documents and returns their content.
//
// Supports reading specific versions with --version (0 means latest,
// which is the default). Multiple paths are concatenated with newlines,
// matching the Unix cat convention.
//
// Line numbering (-n) pads numbers to the width of the highest line
// number so columns stay aligned.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var catSpec = sdk.Command{
	Name: "cat", Desc: `Read and display one or more documents

Prints document content to stdout. Multiple paths are concatenated
with newlines, like Unix cat.`, Usage: "cat [options] <path>...", MCP: true, Flags: []sdk.Flag{
		{Name: "version", Type: "int", Desc: "Read specific version"},
		{Name: "n", Type: "bool", Desc: "Number output lines"},
	},
}

func cat(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, paths, err := sdk.ParseArgs(catSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("cat: %w", err)
	}
	version := flags.Int("version")
	numberLines := flags.Bool("n")

	if len(paths) == 0 {
		return nil, fmt.Errorf("cat: %w", sdk.ErrMissingArg)
	}

	var results []string
	for _, path := range paths {
		content, err := ctx.Documents.Read(path, version)
		if err != nil {
			return nil, fmt.Errorf("cat: %s: %w", path, err)
		}

		text := string(content)
		if numberLines {
			text = addLineNumbers(text)
		}
		results = append(results, text)
	}

	output := strings.Join(results, "\n")
	return sdk.Result{Text: output, Data: output}, nil
}

// addLineNumbers prepends "N  " to each line, where N is right-padded
// to the width of the total line count.
func addLineNumbers(s string) string {
	lines := strings.Split(s, "\n")
	width := len(strconv.Itoa(len(lines)))
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d  %s", width, i+1, line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
