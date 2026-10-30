package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"

	"github.com/jpl-au/llmd/sdk"
)

// display writes an sdk.Response to stdout. JSON output is used when
// jsonOut is true or the response type requires it, in the form
// sdk.JSON gives, as the HTTP server does for Output: json. Returns a
// process exit code.
//
// Markdown responses are rendered via glamour when stdout is an
// interactive terminal and emitted raw otherwise, so pipes, redirects
// and file output get unrendered markdown source while users at a
// terminal get a rendered view. Text and Result are never re-rendered;
// they may already contain lipgloss-styled output that must not be
// touched.
func display(result sdk.Response, jsonOut bool) int {
	if _, always := result.(sdk.Data); jsonOut || always {
		return writeJSON(sdk.JSON(result))
	}
	switch r := result.(type) {
	case sdk.Text:
		if string(r) != "" {
			lipgloss.Fprintln(os.Stdout, string(r))
		}
	case sdk.Markdown:
		if r.Text != "" {
			lipgloss.Fprintln(os.Stdout, renderMarkdown(r.Text))
		}
	case sdk.Result:
		if r.Text != "" {
			lipgloss.Fprintln(os.Stdout, r.Text)
		}
	}
	if r, ok := result.(sdk.Result); ok && r.Note != "" {
		fmt.Fprintln(os.Stderr, r.Note)
	}
	return 0
}

// markdownTheme is the display.theme setting, set by run once the
// config is loaded: "dark" or "light".
var markdownTheme = "dark"

// renderMarkdown returns a glamour-rendered version of the source for
// interactive terminals, falling back to the raw source for
// non-terminal stdout (pipes, redirects, file output) and on render
// errors. This is the single point where llmd decides whether the
// human at the terminal gets pretty markdown.
func renderMarkdown(src string) string {
	if !isTTY() {
		return src
	}
	// The style is set, not detected: asking the terminal for its
	// background costs up to four seconds when it does not answer.
	style := styles.DarkStyle
	if markdownTheme == "light" {
		style = styles.LightStyle
	}
	rendered, err := glamour.Render(src, style)
	if err != nil {
		slog.Debug("rendering markdown", "error", err)
		return src
	}
	return rendered
}

// isTTY reports whether stdout is a terminal. The CLI uses this to
// decide whether to apply human-only formatting (markdown rendering,
// colour, table borders).
func isTTY() bool {
	f, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeCharDevice != 0
}

// writeJSON writes v to stdout as indented JSON and returns the exit
// code.
func writeJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return errorf(false, "%s: encoding JSON: %v", sdk.Program(), err)
	}
	return 0
}

// errorf writes an error to stderr (as JSON if jsonOut, plain text
// otherwise) and returns exit code 1. The message is the error's own
// sentence, which already leads with the command: "cat: p does not
// exist."
func errorf(jsonOut bool, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if jsonOut {
		if e := json.NewEncoder(os.Stderr).Encode(map[string]string{"error": msg}); e != nil {
			slog.Warn("writing error to stderr", "error", e)
		}
	} else {
		// lipgloss drops the colour when stderr is not a terminal.
		if _, e := lipgloss.Fprintln(os.Stderr, highlightCommands(msg, sdk.Program())); e != nil {
			slog.Warn("writing error to stderr", "error", e)
		}
	}
	return 1
}

// commandStyle marks a command an error message tells the reader to
// run, in the terminal's own cyan.
var commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

// highlightCommands colours each "<program> <command>" an error
// message names, program being the binary's name. A command runs from
// the program to the first word that ends a sentence or clause (".",
// ",", ";") or starts what follows it ("to", "first", "then"), as in
// "Run llmd init to create one." The "llmd:" prefix of the CLI's own
// errors is not a command and is left alone.
func highlightCommands(msg, program string) string {
	words := strings.Split(msg, " ")
	for i := 0; i < len(words); i++ {
		if words[i] != program {
			continue
		}
		end, trail := i+1, ""
		for ; end < len(words); end++ {
			w := words[end]
			cut := strings.TrimRight(w, ".,;")
			if cut == "to" || cut == "first" || cut == "then" {
				break
			}
			if cut != w {
				words[end], trail = cut, w[len(cut):]
				end++
				break
			}
		}
		if end == i+1 {
			continue // the program alone names no command
		}
		cmd := commandStyle.Render(strings.Join(words[i:end], " ")) + trail
		words = slices.Replace(words, i, end, cmd)
	}
	return strings.Join(words, " ")
}
