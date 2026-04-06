// Package gemini implements platform.Platform for Gemini CLI.
package gemini

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jpl-au/llmd/assets/platform"
)

// P implements platform.Platform for Gemini CLI.
type P struct{}

func (P) SettingsPath() string               { return "" }
func (P) BudgetArgs(float64) []string        { return nil }
func (P) ResumeArgs(string, string) []string { return nil }

// Stats extracts metrics from Gemini CLI's JSON output.
func (P) Stats(logPath string) (*platform.RunStats, error) {
	data, err := platform.LastJSON(logPath)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}

	var out struct {
		Stats struct {
			Models map[string]struct {
				Tokens struct {
					Input      int `json:"input"`
					Candidates int `json:"candidates"`
				} `json:"tokens"`
			} `json:"models"`
		} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		slog.Debug("parsing gemini output", "error", err)
		return nil, nil
	}

	s := &platform.RunStats{}
	for name, m := range out.Stats.Models {
		s.Model = name
		if m.Tokens.Input > 0 {
			s.InputTokens = &m.Tokens.Input
		}
		if m.Tokens.Candidates > 0 {
			s.OutputTokens = &m.Tokens.Candidates
		}
		break
	}

	if s.Model == "" {
		return nil, nil
	}
	return s, nil
}

// ParseHook translates a Gemini CLI hook payload into a HookEvent.
func (P) ParseHook(payload []byte) (*platform.HookEvent, error) {
	var raw struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("gemini hook: %w", err)
	}

	ev := raw.Event
	switch ev {
	case "SessionStart":
		ev = "session.start"
	case "AfterTool":
		ev = "tool.post"
	default:
		ev = strings.ToLower(ev)
	}

	return &platform.HookEvent{
		Event: ev,
		Meta:  make(map[string]string),
	}, nil
}
