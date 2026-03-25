package agents

import (
	"embed"
	"encoding/json"
	"log/slog"
	"strings"
)

//go:embed profiles/*.json
var profileFiles embed.FS

// Profiles are built-in agent configurations for well-known AI tools,
// loaded from embedded JSON files in the profiles/ directory. Adding
// a new profile is as simple as dropping a JSON file there.
var Profiles map[string]Config

func init() {
	Profiles = make(map[string]Config)

	entries, err := profileFiles.ReadDir("profiles")
	if err != nil {
		slog.Warn("reading embedded agent profiles", "error", err)
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := profileFiles.ReadFile("profiles/" + e.Name())
		if err != nil {
			slog.Warn("reading agent profile", "file", e.Name(), "error", err)
			continue
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			slog.Warn("parsing agent profile", "file", e.Name(), "error", err)
			continue
		}
		Profiles[cfg.Name] = cfg
	}
}

// Profile returns the built-in profile for a known agent, or nil
// if the name is not recognised.
func Profile(name string) *Config {
	cfg, ok := Profiles[name]
	if !ok {
		return nil
	}
	return &cfg
}

// ProfileNames returns the names of all built-in profiles.
func ProfileNames() []string {
	names := make([]string, 0, len(Profiles))
	for name := range Profiles {
		names = append(names, name)
	}
	return names
}
