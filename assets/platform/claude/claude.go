// Package claude implements platform.Platform for Claude Code.
package claude

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jpl-au/llmd/assets/platform"
)

// P implements platform.Platform for Claude Code.
type P struct{}

func (P) SettingsPath() string {
	return ".claude/settings.json"
}

func (P) BudgetArgs(budget float64) []string {
	return []string{fmt.Sprintf("--max-budget-usd=%.2f", budget)}
}

func (P) ResumeArgs(id, prompt string) []string {
	return []string{"--resume", id, "-p", prompt}
}

// Stats extracts metrics from Claude Code's JSON output.
func (P) Stats(logPath string) (*platform.RunStats, error) {
	data, err := platform.LastJSON(logPath)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}

	var out struct {
		TotalCost *float64 `json:"total_cost_usd"`
		SessionID string   `json:"session_id"`
		Usage     struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
		Models map[string]json.RawMessage `json:"modelUsage"`
	}
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		slog.Debug("parsing claude output", "error", err)
		return nil, nil
	}

	s := &platform.RunStats{
		MonetaryCost: out.TotalCost,
		SessionID:    out.SessionID,
	}
	if out.Usage.Input > 0 {
		s.InputTokens = &out.Usage.Input
	}
	if out.Usage.Output > 0 {
		s.OutputTokens = &out.Usage.Output
	}
	for name := range out.Models {
		s.Model = name
		break
	}
	return s, nil
}

// ParseHook translates a Claude Code hook payload into a HookEvent.
func (P) ParseHook(payload []byte) (*platform.HookEvent, error) {
	var raw struct {
		Event      string `json:"hook_event_name"`
		Session    string `json:"session_id"`
		Tool       string `json:"tool_name"`
		Transcript string `json:"transcript_path"`
		CWD        string `json:"cwd"`
		AgentID    string `json:"agent_id"`
		AgentType  string `json:"agent_type"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("claude hook: %w", err)
	}

	meta := make(map[string]string)
	set := func(k, v string) {
		if v != "" {
			meta[k] = v
		}
	}
	set("session_id", raw.Session)
	set("tool_name", raw.Tool)
	set("transcript_path", raw.Transcript)
	set("cwd", raw.CWD)
	set("agent_id", raw.AgentID)
	set("agent_type", raw.AgentType)

	return &platform.HookEvent{
		Event: event(raw.Event),
		Meta:  meta,
	}, nil
}

// event maps Claude Code's PascalCase event names to dotted convention.
func event(name string) string {
	switch name {
	case "SessionStart":
		return "session.start"
	case "SessionEnd":
		return "session.end"
	case "PreToolUse":
		return "tool.pre"
	case "PostToolUse":
		return "tool.post"
	case "Stop":
		return "session.stop"
	case "TaskCompleted":
		return "task.completed"
	case "SubagentStop":
		return "subagent.stop"
	default:
		return strings.ToLower(name)
	}
}
