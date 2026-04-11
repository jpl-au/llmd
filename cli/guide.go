// guide.go provides built-in documentation for humans and LLMs.
//
// Guide content is markdown; the host's renderer turns it into a
// glamour-rendered view for interactive terminals and emits raw
// markdown for pipes, --json, MCP, and HTTP. The command itself does
// no rendering - that decision lives once at the host boundary, so
// guide stays in sync with cat and any other markdown-emitting
// command automatically.
//
// Usage:
//
//	llmd guide              Full command guide
//	llmd guide <topic>      Help on a specific topic (e.g. "cat", "grep")

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/guide"
	"github.com/jpl-au/llmd/sdk"
)

var guideSpec = sdk.Command{
	Name: "guide", Desc: `Read built-in documentation on any command or topic

Without a topic, shows the overview with all available commands.
Topics include any command name as well as workflow, install, and
other guides. Pipe the output or use --json to get raw markdown.`, Usage: "guide [topic]", MCP: true,
}

func guideCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	_, positional, err := sdk.ParseArgs(guideSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("guide: %w", err)
	}
	var topic string
	if len(positional) > 0 {
		topic = positional[0]
	}

	content, err := guide.Get(topic)
	if err != nil {
		topics, listErr := guide.List()
		if listErr != nil {
			return nil, fmt.Errorf("guide: %w: %s", sdk.ErrNotFound, topic)
		}
		return nil, fmt.Errorf("guide: %w: %s\n\nAvailable: %s",
			sdk.ErrNotFound, topic, strings.Join(topics, ", "))
	}

	return sdk.Markdown{Text: content}, nil
}
