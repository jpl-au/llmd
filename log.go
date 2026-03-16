package main

import (
	"log/slog"
	"os"

	"github.com/jpl-au/llmd/internal/config"
)

// initLog configures the process-wide slog logger. By default the
// level is Warn (quiet CLI). --verbose overrides to Debug. Config
// keys log.level and log.format provide persistent control. --json
// implies JSON-formatted logs so structured output stays
// machine-readable.
func initLog(cfg config.Config, jsonOut, verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	} else {
		switch cfg.Log.Level {
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

	format := cfg.Log.Format
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
