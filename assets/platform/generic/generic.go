// Package generic implements platform.Platform as a best-effort
// fallback for unknown agent platforms.
package generic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/assets/platform"
)

// P implements platform.Platform for unknown agents.
type P struct{}

func (P) SettingsPath() string                     { return "" }
func (P) BudgetArgs(float64) []string              { return nil }
func (P) ResumeArgs(string, string) []string       { return nil }
func (P) Stats(string) (*platform.RunStats, error) { return nil, nil }

// ParseHook does a best-effort parse of an unknown platform's hook
// payload, looking for common field names.
func (P) ParseHook(payload []byte) (*platform.HookEvent, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("hook: %w", err)
	}

	ev := platform.StringField(raw, "event", "hook_event_name", "type")
	id := platform.StringField(raw, "task_id", "taskId", "task")
	content := platform.StringField(raw, "content", "summary", "message", "text")

	meta := make(map[string]string)
	for k, v := range raw {
		if s, ok := v.(string); ok {
			meta[k] = s
		}
	}

	return &platform.HookEvent{
		Event:   strings.ToLower(ev),
		TaskID:  id,
		Content: content,
		Meta:    meta,
	}, nil
}
