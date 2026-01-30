//go:build wasip1

package sdk

// Result is the result of a command execution.
//
// Commands return either TextResult for plain text output or JSONResult
// for structured data. The host converts the result to the appropriate
// format based on context (CLI output, MCP response, etc.).
type Result interface {
	isResult()
}

// TextResult represents plain text output from a command.
//
// Use TextResult when the output is human-readable text that doesn't
// need structured parsing. Appropriate for most CLI-style commands.
//
// Example:
//
//	return sdk.TextResult("Document created successfully"), nil
type TextResult string

func (TextResult) isResult() {}

// JSONResult represents structured JSON output from a command.
//
// Use JSONResult when the output is structured data that callers may
// want to parse programmatically. Useful for MCP commands where AI
// assistants need to process the results.
//
// Example:
//
//	return sdk.JSONResult{Data: map[string]any{
//	    "path":    doc.Path,
//	    "version": doc.Version,
//	}}, nil
type JSONResult struct {
	// Data is the structured data to return.
	Data any `json:"data"`
}

func (JSONResult) isResult() {}

// RichResult provides both human-readable text and structured data.
//
// Use RichResult when a command produces data that has both a natural text
// representation (for CLI users) and a structured form (for scripts and
// automation). The host selects which representation to use based on CLI
// flags (--json uses Data, default uses Text).
//
// This is the preferred result type for data-producing commands like ls,
// search, or grep. It allows the same command to serve both human users
// and automated tooling without the plugin needing to know the output format.
//
// Example:
//
//	paths, _ := sdk.Host.List(prefix)
//	return sdk.RichResult{
//	    Text: strings.Join(paths, "\n"),
//	    Data: paths,
//	}, nil
type RichResult struct {
	// Text is the human-readable output shown by default in the CLI.
	// This should be formatted for terminal display (e.g., one item per line).
	Text string

	// Data is the structured data returned when --json flag is used.
	// This will be JSON-encoded and pretty-printed by the host.
	Data any
}

func (RichResult) isResult() {}
