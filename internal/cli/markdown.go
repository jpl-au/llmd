package cli

import (
	"os"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown for terminal display.
// Returns the input unchanged if stdout is not a TTY.
func renderMarkdown(md string) string {
	if md == "" {
		return md
	}

	stat, err := os.Stdout.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return md
	}

	out, err := glamour.Render(md, "auto")
	if err != nil {
		return md
	}
	return out
}
