// Package cli provides the command-line interface for llmd.
package cli

import "fmt"

// OutputFormat specifies the output format for command results.
//
// The zero value is invalid, which catches uninitialised values.
type OutputFormat int

const (
	_ OutputFormat = iota // zero value invalid
	OutputText
	OutputJSON
	OutputMarkdown
)

// String returns the format name.
func (f OutputFormat) String() string {
	switch f {
	case OutputText:
		return "text"
	case OutputJSON:
		return "json"
	case OutputMarkdown:
		return "md"
	default:
		return "unknown"
	}
}

// ParseOutputFormat parses a format string.
//
// Valid values: "text", "json", "md", "markdown".
// Returns an error for unrecognised formats.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch s {
	case "text":
		return OutputText, nil
	case "json":
		return OutputJSON, nil
	case "md", "markdown":
		return OutputMarkdown, nil
	default:
		return 0, fmt.Errorf("unknown output format: %s", s)
	}
}
