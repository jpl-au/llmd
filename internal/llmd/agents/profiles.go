package agents

// Profiles are built-in agent configurations for well-known AI tools.
// When a user runs "llmd agent add claude-code", llmd already knows
// the binary, arguments, and prompt patterns. The user does not need
// to teach llmd how to invoke common agents.
var Profiles = map[string]Config{
	"claude-code": {
		Name:    "claude-code",
		Command: "claude",
		Args:    []string{"-p", "{{.Prompt}}", "--output-format", "json"},
	},
	"gemini": {
		Name:    "gemini",
		Command: "gemini",
		Args:    []string{"-p", "{{.Prompt}}"},
	},
	"aider": {
		Name:    "aider",
		Command: "aider",
		Args:    []string{"--message", "{{.Prompt}}"},
	},
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
