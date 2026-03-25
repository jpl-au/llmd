package agents

import (
	"embed"
	"log/slog"
	"strings"
)

//go:embed templates/*.md
var templateFiles embed.FS

// DefaultTemplates maps role names to their built-in prompt templates,
// loaded from embedded markdown files in the templates/ directory.
// Adding a new default role template is as simple as dropping an .md
// file there.
var DefaultTemplates map[string]string

func init() {
	DefaultTemplates = make(map[string]string)

	entries, err := templateFiles.ReadDir("templates")
	if err != nil {
		slog.Warn("reading embedded agent templates", "error", err)
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := templateFiles.ReadFile("templates/" + e.Name())
		if err != nil {
			slog.Warn("reading agent template", "file", e.Name(), "error", err)
			continue
		}
		role := strings.TrimSuffix(e.Name(), ".md")
		DefaultTemplates[role] = string(data)
	}
}

// DefaultTemplate returns the built-in template for a role, or empty
// string if no default exists.
func DefaultTemplate(role string) string {
	return DefaultTemplates[role]
}
