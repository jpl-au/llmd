package gemini

import "fmt"

// HookConfig returns a Gemini CLI hook configuration that calls llmd
// directly via CLI. Gemini does not support HTTP hooks natively.
func HookConfig(_, author string) string {
	return fmt.Sprintf(`{
  "hooks": [
    {
      "event": "SessionStart",
      "type": "command",
      "command": "llmd --author %s queue ls --limit 5"
    },
    {
      "event": "AfterTool",
      "type": "command",
      "command": "llmd --author %s queue peek"
    }
  ]
}
`, author, author)
}
