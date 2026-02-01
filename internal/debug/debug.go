// Package debug provides debug logging for llmd.
//
// Debug logging is controlled by the Debug variable, which can be set at
// build time using ldflags:
//
//	go build -ldflags "-X github.com/jpl-au/llmd/internal/debug.Debug=true"
//
// When Debug is "true", debug messages are logged to stderr using slog.
// When Debug is anything else (default ""), debug logging is disabled.
package debug

import (
	"log/slog"
	"os"
)

// Debug controls whether debug logging is enabled.
// Set at build time via ldflags.
var Debug string

var logger *slog.Logger

func init() {
	if Debug == "true" {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
}

// Log logs a debug message with optional key-value pairs.
// Does nothing if debug mode is disabled.
func Log(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	}
}

// Enabled returns true if debug logging is enabled.
func Enabled() bool {
	return logger != nil
}
