package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/jpl-au/llmd/sdk"
)

// display writes an sdk.Response to stdout. JSON output is used when
// jsonOut is true or the response type requires it. Returns a process
// exit code.
func display(result sdk.Response, jsonOut bool) int {
	switch r := result.(type) {
	case sdk.Text:
		if string(r) != "" {
			lipgloss.Println(string(r))
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
