package assets

import (
	"strings"

	"github.com/jpl-au/llmd/assets/platform"
	"github.com/jpl-au/llmd/assets/platform/claude"
	"github.com/jpl-au/llmd/assets/platform/gemini"
	"github.com/jpl-au/llmd/assets/platform/generic"
)

// Platform returns the Platform for the named agent. Unknown agents
// get a best-effort fallback.
func (*agentAssets) Platform(name string) platform.Platform {
	switch {
	case strings.Contains(name, "claude"):
		return claude.P{}
	case strings.Contains(name, "gemini"):
		return gemini.P{}
	default:
		return generic.P{}
	}
}
