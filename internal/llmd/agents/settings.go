package agents

import (
	"embed"
	"log/slog"
	"strings"
)

//go:embed settings/*.json
var settingsFiles embed.FS

// DefaultSettings maps agent names to their default settings content,
// loaded from embedded JSON files in the settings/ directory. These
// are written to the worktree during spawn to configure the agent's
// runtime environment (e.g. permissions for Claude Code).
var DefaultSettings map[string]string

func init() {
	DefaultSettings = make(map[string]string)

	entries, err := settingsFiles.ReadDir("settings")
	if err != nil {
		slog.Warn("reading embedded agent settings", "error", err)
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := settingsFiles.ReadFile("settings/" + e.Name())
		if err != nil {
			slog.Warn("reading agent settings", "file", e.Name(), "error", err)
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		DefaultSettings[name] = string(data)
	}
}
