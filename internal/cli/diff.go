package cli

import (
	"os"
	"strings"

	"github.com/fatih/color"
)

// colorizeDiff applies ANSI colors to unified diff output.
// Returns the input unchanged if stdout is not a TTY.
func colorizeDiff(diff string) string {
	if diff == "" {
		return diff
	}

	// Check if stdout is a TTY
	stat, err := os.Stdout.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return diff // Not a TTY, return uncolored
	}

	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	var result strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "---"):
			result.WriteString(bold(line))
		case strings.HasPrefix(line, "+++"):
			result.WriteString(bold(line))
		case strings.HasPrefix(line, "@@"):
			result.WriteString(cyan(line))
		case strings.HasPrefix(line, "-"):
			result.WriteString(red(line))
		case strings.HasPrefix(line, "+"):
			result.WriteString(green(line))
		default:
			result.WriteString(line)
		}
		result.WriteString("\n")
	}
	// Remove trailing newline added by loop
	s := result.String()
	if strings.HasSuffix(diff, "\n") {
		return s
	}
	return strings.TrimSuffix(s, "\n")
}
