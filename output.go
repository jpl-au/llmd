package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/jpl-au/llmd/sdk"
)

// display writes an sdk.Response to stdout. JSON output is used when
// jsonOut is true or the response type requires it. Returns a process
// exit code.
//
// Markdown responses are rendered via glamour when stdout is an
// interactive terminal and emitted raw otherwise, so pipes, redirects
// and file output get unrendered markdown source while users at a
// terminal get a rendered view. Text and Result are never re-rendered;
// they may already contain lipgloss-styled output that must not be
// touched.
func display(result sdk.Response, jsonOut bool) int {
	switch r := result.(type) {
	case sdk.Text:
		if string(r) != "" {
			lipgloss.Println(string(r))
		}
	case sdk.Markdown:
		if jsonOut && r.Data != nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(r.Data); err != nil {
				return errorf(false, "encoding JSON: %v", err)
			}
		} else if r.Text != "" {
			lipgloss.Println(renderMarkdown(r.Text))
		}
	case sdk.Result:
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(r.Data); err != nil {
				return errorf(false, "encoding JSON: %v", err)
			}
		} else if r.Text != "" {
			lipgloss.Println(r.Text)
		}
	case sdk.Data:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r.V); err != nil {
			return errorf(false, "encoding JSON: %v", err)
		}
	}
	return 0
}

// renderMarkdown returns a glamour-rendered version of the source for
// interactive terminals, falling back to the raw source for
// non-terminal stdout (pipes, redirects, file output) and on render
// errors. This is the single point where llmd decides whether the
// human at the terminal gets pretty markdown.
func renderMarkdown(src string) string {
	if !isTTY() {
		return src
	}
	rendered, err := glamour.Render(src, "dark")
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

// errorf writes an error to stderr (as JSON if jsonOut, plain text
// otherwise) and returns exit code 1.
func errorf(jsonOut bool, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if jsonOut {
		if e := json.NewEncoder(os.Stderr).Encode(map[string]string{"error": msg}); e != nil {
			slog.Warn("writing error to stderr", "error", e)
		}
	} else {
		if _, e := fmt.Fprintf(os.Stderr, "error: %s\n", msg); e != nil {
			slog.Warn("writing error to stderr", "error", e)
		}
	}
	return 1
}
