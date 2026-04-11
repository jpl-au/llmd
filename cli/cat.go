package cli

// cat reads one or more documents and returns their content.
//
// Supports reading specific versions with --version (0 means latest,
// which is the default). Multiple paths are concatenated with newlines,
// matching the Unix cat convention.
//
// The joined output is returned as sdk.Markdown so the host renders
// it through glamour for interactive terminals and emits raw markdown
// source for pipes, --json, MCP, HTTP and any other consumer. Cat
// itself does no rendering - that lives in one place at the host
// boundary so every markdown-emitting command behaves consistently.
// Use -n to number lines, which produces a sdk.Result instead since
// numbered output is no longer pure markdown.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var catSpec = sdk.Command{
	Name: "cat", Desc: `Read and display one or more documents

Prints document content to stdout. Multiple paths are concatenated
with newlines, like Unix cat. On an interactive terminal the output
is rendered as markdown; pipes, redirects and --json see the raw
source.`, Usage: "cat [options] <path>...", MCP: true, Flags: []sdk.Flag{
		{Name: "version", Type: "int", Desc: "Read specific version"},
		{Name: "n", Type: "bool", Desc: "Number output lines (disables markdown rendering)"},
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

	// Numbered output is no longer pure markdown - the line-number
	// prefix would confuse glamour - so it ships as Result instead.
	if numberLines {
		return sdk.Result{Text: output, Data: output}, nil
	}
	return sdk.Markdown{Text: output, Data: output}, nil
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
