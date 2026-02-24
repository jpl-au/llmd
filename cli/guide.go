// guide.go provides built-in documentation for humans and LLMs.
//
// Terminal output is rendered through glamour for readability.
// Piped output and --raw give raw markdown, suitable for LLMs and scripts.
//
// Usage:
//
//	llmd guide              Full command guide
//	llmd guide <topic>      Help on a specific topic (e.g. "cat", "grep")

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/jpl-au/llmd/guide"
	"github.com/jpl-au/llmd/sdk"
)

func guideCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var topic string
	var raw bool

	for _, arg := range args {
		if arg == "--raw" {
			raw = true
		} else {
			topic = arg
		}
	}

	content, err := guide.Get(topic)
	if err != nil {
		topics, listErr := guide.List()
		if listErr != nil {
			return nil, err
		}
		return nil, fmt.Errorf("guide: unknown topic: %s\n\nAvailable: %s",
			topic, strings.Join(topics, ", "))
	}

	if !raw && isTTY() {
		rendered, err := glamour.Render(content, "dark")
		if err == nil {
			return sdk.Text(rendered), nil
		}
	}

	return sdk.Text(content), nil
}

// isTTY reports whether stdout is a terminal.
func isTTY() bool {
	f, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeCharDevice != 0
}
