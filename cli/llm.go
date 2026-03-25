// llm.go provides a quick command reference for AI agents.
// Agents call "llmd llm" to get oriented with available commands.

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/sdk"
)

var llmSpec = sdk.Command{
	Name: "llm", Desc: "Quick command reference for AI agents", Usage: "llm", MCP: true,
}

func llm(ctx sdk.Context, args []string) (sdk.Response, error) {
	content, err := assets.Guide.Get("llm")
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	return sdk.Text(content), nil
}
