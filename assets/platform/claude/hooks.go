package claude

import "fmt"

// HookConfig returns a Claude Code settings.json hooks snippet
// configured to POST lifecycle events to the llmd HTTP server.
func HookConfig(addr, author string) string {
	url := "http://" + addr + "/hook"
	return fmt.Sprintf(`{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "http",
            "url": %q,
            "headers": {"Author": %q}
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "http",
            "url": %q,
            "headers": {"Author": %q}
          }
        ]
      }
    ],
    "TaskCompleted": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "http",
            "url": %q,
            "headers": {"Author": %q}
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "mcp__llmd.*",
        "hooks": [
          {
            "type": "http",
            "url": %q,
            "headers": {"Author": %q}
          }
        ]
      }
    ]
  }
}
`, url, author, url, author, url, author, url, author)
}
