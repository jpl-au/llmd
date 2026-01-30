// Package cli provides the command-line interface for llmd.
package cli

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
