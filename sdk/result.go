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
