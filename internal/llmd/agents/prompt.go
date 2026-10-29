package agents

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	texttemplate "text/template"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/sdk"
)

// PromptData holds the values substituted into prompt templates.
type PromptData struct {
	Key        string
	Title      string
	Branch     string
	AssignedTo string
	Agent      string
	URL        string // HTTP API URL of the running llmd serve
	Auth       string // curl flags that send server.token, with a leading space; empty without one
	SpecPath   string
	OnSuccess  string // column to move to on success
	OnFailure  string // column to move to on failure
}

// BuildPrompt resolves a prompt template for the agent/role, renders
// it with the given data, and returns the result. Agents reach llmd
// through the HTTP server, so every template is written for it. The
// resolution chain is:
//
//  1. .llmd/agents/<name>/<role>.md
//  2. .llmd/agents/default/<role>.md
//  3. Built-in embedded template
func (a *Agents) BuildPrompt(_ context.Context, cfg *sdk.AgentConfig, data PromptData) string {
	role := cfg.Role
	if role == "" {
		role = "developer"
	}

	tmplContent := a.resolveTemplate(cfg.Name, role)

	tmpl, err := texttemplate.New("prompt").Parse(tmplContent)
	if err != nil {
		slog.Warn("parsing prompt template", "agent", cfg.Name, "role", role, "error", err)
		return fallbackPrompt(cfg.Name, data)
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		slog.Warn("executing prompt template", "agent", cfg.Name, "role", role, "error", err)
		return fallbackPrompt(cfg.Name, data)
	}

	return b.String()
}

// resolveTemplate walks the fallback chain to find a prompt template.
func (a *Agents) resolveTemplate(name, role string) string {
	ctx := context.Background()

	// Template from disk.
	if content, _, err := a.Prompt(ctx, name, role); err == nil && content != "" {
		return content
	}

	// Fall back to built-in embedded templates.
	slog.Debug("no stored prompt template, using built-in", "agent", name, "role", role)
	if t := assets.Agent.Template(role); t != "" {
		return t
	}

	return fmt.Sprintf("You are agent %s. Complete the assigned task.", name)
}

func fallbackPrompt(agent string, data PromptData) string {
	return fmt.Sprintf("You are agent %s working on task %s: %s. API at %s",
		agent, data.Key, data.Title, data.URL)
}
