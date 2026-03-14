package main

import (
	"log/slog"
	"os"
)

// initLog configures the process-wide slog logger. By default the
// level is Warn (quiet CLI). --verbose overrides to Debug. Config
// keys log_level and log_format provide persistent control. --json
// implies JSON-formatted logs so structured output stays
// machine-readable.
func initLog(cfg map[string]string, jsonOut, verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	} else if v, ok := cfg["log_level"]; ok {
		switch v {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	format := cfg["log_format"]
	if jsonOut {
		format = "json"
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}
