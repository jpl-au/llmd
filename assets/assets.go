// Package assets provides embedded agent resources that ship with
// the llmd binary. Agent profiles, runtime settings, and prompt
// templates are accessible through [Agent].
//
// Files live under assets/ in the repository root:
//
//	assets/
//	  agents/
//	    claude-code/    config.json, settings.json
//	    gemini/         config.json
//	    aider/          config.json
//	    default/        developer.md, auditor.md, ...
package assets

import (
	"embed"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

//go:embed agents/*/*
var files embed.FS

// Agent provides access to embedded agent defaults.
var Agent = &agentAssets{}

// profiles, settings, and templates are populated once at init.
var (
	profiles  map[string]sdk.AgentConfig
	settings  map[string]string
	templates map[string]string
)

func init() {
	profiles = make(map[string]sdk.AgentConfig)
	settings = make(map[string]string)
	templates = make(map[string]string)

	dirs, err := files.ReadDir("agents")
	if err != nil {
		slog.Warn("reading embedded agent assets", "error", err)
		return
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		name := dir.Name()

		entries, err := files.ReadDir("agents/" + name)
		if err != nil {
			slog.Warn("reading agent directory", "name", name, "error", err)
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := "agents/" + name + "/" + e.Name()
			data, err := files.ReadFile(path)
			if err != nil {
				slog.Warn("reading agent file", "path", path, "error", err)
				continue
			}

			switch {
			case e.Name() == "config.json":
				var cfg sdk.AgentConfig
				if err := json.Unmarshal(data, &cfg); err != nil {
					slog.Warn("parsing agent config", "path", path, "error", err)
					continue
				}
				profiles[cfg.Name] = cfg

			case e.Name() == "settings.json":
				settings[name] = string(data)

			case strings.HasSuffix(e.Name(), ".md"):
				role := strings.TrimSuffix(e.Name(), ".md")
				if name == "default" {
					templates[role] = string(data)
				} else {
					templates[name+"/"+role] = string(data)
				}
			}
		}
	}
}

// agentAssets provides access to embedded agent defaults.
type agentAssets struct{}

// Profile returns the built-in profile for a known agent, or nil
// if the name is not recognised.
func (*agentAssets) Profile(name string) *sdk.AgentConfig {
	cfg, ok := profiles[name]
	if !ok {
		return nil
	}
	return &cfg
}

// ProfileNames returns the names of all built-in profiles.
func (*agentAssets) ProfileNames() []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	return names
}

// Settings returns the default runtime settings for an agent, or
// empty string if none exist.
func (*agentAssets) Settings(name string) string {
	return settings[name]
}

// Template returns the default prompt template for a role. Returns
// empty string if no default exists.
func (*agentAssets) Template(role string) string {
	return templates[role]
}

// Templates returns all default templates keyed by role name. Only
// includes templates from the default/ directory.
func (*agentAssets) Templates() map[string]string {
	out := make(map[string]string)
	for role, content := range templates {
		if !strings.Contains(role, "/") {
			out[role] = content
		}
	}
	return out
}
