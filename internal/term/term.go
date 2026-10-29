// Package term provides terminal detection and stdin reading utilities.
//
// These are used by the CLI entry point to distinguish interactive
// (human at a terminal) from non-interactive (piped, script, LLM)
// invocations.
package term

import (
	"io"
	"log/slog"
	"os"
)

// Interactive reports whether stdout is connected to a terminal.
func Interactive() bool {
	f, err := os.Stdout.Stat()
	if err != nil {
		slog.Debug("cannot stat stdout, assuming non-interactive", "error", err)
		return false
	}
	return f.Mode()&os.ModeCharDevice != 0
}

// Stdin returns the process's stdin as an input source when it is a
// pipe or a redirected file, or nil for an interactive terminal. It
// reads nothing: commands read it only if they need input, through
// sdk.Context.Input or OptionalInput.
func Stdin() io.Reader {
	stat, err := os.Stdin.Stat()
	if err != nil {
		slog.Debug("cannot stat stdin", "error", err)
		return nil
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	return os.Stdin
}
