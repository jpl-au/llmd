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
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/sdk"
)

var guideSpec = sdk.Command{
	Name: "guide", Desc: `Read built-in documentation on any command or topic

Without a topic, shows the overview with all available commands.
Topics include any command name as well as workflow, install, and
other guides. Use --raw for unrendered markdown.`, Usage: "guide [--raw] [topic]", MCP: true, Flags: []sdk.Flag{
		{Name: "raw", Type: "bool", Desc: "Output raw markdown without rendering"},
	},
}

func guideCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(guideSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("guide: %w", err)
	}
	raw := flags.Bool("raw")
	var topic string
	if len(positional) > 0 {
		topic = positional[0]
	}

	content, err := assets.Guide.Get(topic)
	if err != nil {
		topics, listErr := assets.Guide.List()
		if listErr != nil {
			return nil, fmt.Errorf("guide: %w: %s", sdk.ErrNotFound, topic)
		}
		return nil, fmt.Errorf("guide: %w: %s\n\nAvailable: %s",
			sdk.ErrNotFound, topic, strings.Join(topics, ", "))
	}

	if !raw && isTTY() {
		rendered, err := glamour.Render(content, "dark")
		if err == nil {
			return sdk.Text(rendered), nil
		}
	}

	return sdk.Text(content), nil
}
