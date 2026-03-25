package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// Config is the internal representation of an agent configuration.
// Stored as a JSON document at agents/<name>/config.
type Config struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
	Role      string   `json:"role,omitempty"`
	MaxBudget float64  `json:"max_budget,omitempty"`
}

// Register writes an agent configuration as a document. If the config
// document already exists, it is versioned (standard document behaviour).
func (a *Agents) Register(ctx context.Context, cfg Config, author string) error {
	if cfg.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if cfg.Command == "" {
		return fmt.Errorf("agent command is required")
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding agent config: %w", err)
	}

	path := ConfigPath(cfg.Name)
	orig := documents.WriteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	}
	_, err = a.docs.Write(ctx, path, string(data), orig)
	if err != nil {
		return err
	}

	// Seed default prompt templates if they don't already exist.
	for role, content := range assets.Agent.Templates() {
		a.seedPrompt(ctx, cfg.Name, role, content, orig)
	}

	// Seed default runtime settings if available.
	if s := assets.Agent.Settings(cfg.Name); s != "" {
		a.seedDoc(ctx, SettingsPath(cfg.Name), s, orig)
	}

	return nil
}

// seedPrompt writes a default prompt template only if the document
// does not already exist.
func (a *Agents) seedPrompt(ctx context.Context, name, role, content string, opts documents.WriteOptions) {
	a.seedDoc(ctx, PromptPath(name, role), content, opts)
}

// seedDoc writes a document only if it does not already exist.
func (a *Agents) seedDoc(ctx context.Context, path, content string, opts documents.WriteOptions) {
	if _, err := a.docs.Read(ctx, path); err == nil {
		return
	}
	a.docs.Write(ctx, path, content, opts)
}

// Settings reads the runtime settings for an agent. Returns empty
// string if no settings document exists.
func (a *Agents) Settings(ctx context.Context, name string) string {
	path := SettingsPath(name)
	doc, err := a.docs.Read(ctx, path)
	if err != nil {
		return ""
	}
	return doc.Content
}

// Remove soft-deletes an agent's config document.
func (a *Agents) Remove(ctx context.Context, name, author string) error {
	path := ConfigPath(name)
	return a.docs.Delete(ctx, path, documents.DeleteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

// Agent reads a single agent configuration by name.
func (a *Agents) Agent(ctx context.Context, name string) (*Config, error) {
	path := ConfigPath(name)
	doc, err := a.docs.Read(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	var cfg Config
	if err := json.Unmarshal([]byte(doc.Content), &cfg); err != nil {
		return nil, fmt.Errorf("decoding agent config at %s: %w", path, err)
	}
	return &cfg, nil
}

// Agents returns all registered agent configurations by listing
// documents under the agents/ prefix and reading each config.
func (a *Agents) Agents(ctx context.Context) ([]Config, error) {
	docs, err := a.docs.List(ctx, documents.ListOptions{Prefix: PathPrefix})
	if err != nil {
		return nil, err
	}

	// Collect unique agent names from config document paths.
	seen := make(map[string]bool)
	var configs []Config
	for _, d := range docs {
		if !strings.HasSuffix(d.Path, "/"+ConfigDoc) {
			continue
		}
		// Extract agent name: agents/<name>/config → <name>
		name := strings.TrimPrefix(d.Path, PathPrefix)
		name = strings.TrimSuffix(name, "/"+ConfigDoc)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		cfg, err := a.Agent(ctx, name)
		if err != nil {
			continue
		}
		configs = append(configs, *cfg)
	}
	return configs, nil
}

// Prompt reads a prompt template document. It follows the fallback
// chain: agents/<name>/<role> → agents/default/<role>.
// Returns the template content and the path it was found at.
func (a *Agents) Prompt(ctx context.Context, name, role string) (string, string, error) {
	// Try agent-specific template first.
	path := PromptPath(name, role)
	doc, err := a.docs.Read(ctx, path)
	if err == nil {
		return doc.Content, path, nil
	}

	// Fall back to default template.
	fallback := PromptPath("default", role)
	doc, err = a.docs.Read(ctx, fallback)
	if err == nil {
		return doc.Content, fallback, nil
	}

	return "", "", fmt.Errorf("%w: no prompt template for %s/%s", ErrNotFound, name, role)
}

// WritePrompt writes a prompt template document.
func (a *Agents) WritePrompt(ctx context.Context, name, role, content, author string) error {
	path := PromptPath(name, role)
	_, err := a.docs.Write(ctx, path, content, documents.WriteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
	return err
}
