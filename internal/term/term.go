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
	"time"
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

// ReadStdin reads piped input if present, or returns nil for
// interactive terminals. For pipes, a short timeout avoids blocking
// on empty pipes (e.g. certain process managers).
func ReadStdin() []byte {
	f := os.Stdin
	stat, err := f.Stat()
	if err != nil {
		slog.Debug("cannot stat stdin", "error", err)
		return nil
	}

	// Interactive terminal - no piped input.
	if stat.Mode()&os.ModeCharDevice != 0 {
		return nil
	}

	// Regular file (e.g. redirected from disk) - read directly.
	if stat.Mode().IsRegular() {
		data, err := io.ReadAll(f)
		if err != nil {
			slog.Warn("reading stdin", "error", err)
			return nil
		}
		return data
	}

	// Pipe - read with timeout to avoid hanging on empty pipes.
	done := make(chan []byte, 1)
	go func() {
		data, err := io.ReadAll(f)
		if err != nil {
			slog.Warn("reading stdin pipe", "error", err)
		}
		done <- data
	}()
	select {
	case data := <-done:
		return data
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}
