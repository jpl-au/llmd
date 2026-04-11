package cli

// cat reads one or more documents and returns their content.
//
// llmd is an AI-first tool, and cat is the read half of the
// grep+read workflow agents depend on. After grep narrows down which
// document and roughly where the match is, the agent uses cat to
// fetch a bounded slice of lines - never the whole document unless
// it explicitly asks. --offset picks the 1-indexed start line,
// --limit caps the number of lines returned. Without either flag cat
// returns the whole document, matching Unix cat for small files.
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
source.

Use --offset and --limit to read a line range of a long document
without loading the whole file. This is the read half of the
grep+read workflow: grep narrows down where a match lives, cat
fetches just that slice.`, Usage: "cat [options] <path>...", MCP: true, Flags: []sdk.Flag{
		{Name: "version", Type: "int", Desc: "Read specific version"},
		{Name: "offset", Type: "int", Desc: "Start reading from this 1-indexed line"},
		{Name: "limit", Type: "int", Desc: "Maximum number of lines to return"},
		{Name: "n", Type: "bool", Desc: "Number output lines (disables markdown rendering)"},
	},
}

func cat(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, paths, err := sdk.ParseArgs(catSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("cat: %w", err)
	}
	opts := sdk.ReadOpts{
		Version: flags.Int("version"),
		Offset:  flags.Int("offset"),
		Limit:   flags.Int("limit"),
	}
	numberLines := flags.Bool("n")

	if len(paths) == 0 {
		return nil, fmt.Errorf("cat: %w", sdk.ErrMissingArg)
	}

	// When offset is set, line numbers from addLineNumbers should
	// match the source document, not restart from 1 inside the
	// sliced range. Agents piecing together a long doc via repeated
	// cat --offset calls need stable line numbers to correlate with
	// grep results.
	startLine := opts.Offset
	if startLine < 1 {
		startLine = 1
	}

	var results []string
	for _, path := range paths {
		content, err := ctx.Documents.Read(path, opts)
		if err != nil {
			return nil, fmt.Errorf("cat: %s: %w", path, err)
		}

		text := string(content)
		if numberLines {
			text = addLineNumbers(text, startLine)
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
// to the width of the highest line number. start is the 1-indexed
// line number to assign to the first line, so a caller that sliced
// lines 100-110 out of a longer document can pass start=100 and see
// stable line numbers that match the source.
func addLineNumbers(s string, start int) string {
	lines := strings.Split(s, "\n")
	width := len(strconv.Itoa(start + len(lines) - 1))
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d  %s", width, start+i, line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
