package claude

import (
	"encoding/json"

	"github.com/jpl-au/llmd/assets/platform"
)

// HookConfig returns the hooks section of a Claude Code settings.json
// that runs "llmd hook" for session start and end, task completion,
// and tool use on llmd's MCP tools. The payload Claude Code passes on
// stdin is what the hook command parses.
func HookConfig(author string) string {
	hook := []map[string]string{{"type": "command", "command": platform.HookCommand(author)}}
	entry := func(matcher string) []map[string]any {
		return []map[string]any{{"matcher": matcher, "hooks": hook}}
	}
	cfg := map[string]any{
		"hooks": map[string]any{
			"SessionStart":  entry("startup"),
			"SessionEnd":    entry(""),
			"TaskCompleted": entry(""),
			"PostToolUse":   entry("mcp__llmd.*"),
		},
	}
	out, _ := json.MarshalIndent(cfg, "", "  ")
	return string(out) + "\n"
}
