package gemini

import (
	"encoding/json"

	"github.com/jpl-au/llmd/assets/platform"
)

// HookConfig returns a Gemini CLI hook configuration that runs
// "llmd hook" at session start and after each tool use. Gemini passes
// its event payload on stdin, which the hook command parses.
func HookConfig(author string) string {
	cmd := platform.HookCommand(author)
	cfg := map[string]any{
		"hooks": []map[string]string{
			{"event": "SessionStart", "type": "command", "command": cmd},
			{"event": "AfterTool", "type": "command", "command": cmd},
		},
	}
	out, _ := json.MarshalIndent(cfg, "", "  ")
	return string(out) + "\n"
}
