package search

// Result represents a document that matched a search query.
// A single document may have multiple Matches depending on the Mode.
type Result struct {
	// Path is the document path (e.g., "docs/readme").
	Path string

	// Key is the internal document identifier.
	Key string

	// Matches contains the specific matches within this document.
	// Empty for ModePaths. For ModeFull, contains one Match with
	// the entire document. For other modes, contains one Match
	// per matching line/section/snippet.
	Matches []Match
}

// Match represents a specific match location within a document.
// Which fields are populated depends on the search Mode:
//
//	ModeFull:     Text contains entire document, Line=1
//	ModePaths:    (no Match objects returned)
//	ModeLines:    Line, Column, Text, Before, After populated
//	ModeSections: Line, Text, Section populated
//	ModeSnippets: Text contains highlighted snippet
type Match struct {
	// Line is the 1-indexed line number where the match starts.
	// For ModeFull, this is always 1. For ModeSections, this is
	// the line where the section heading appears.
	Line int

	// Column is the 0-indexed byte position within the line where
	// the first matching term starts. Only set for ModeLines.
	Column int

	// Text contains the matched content. The meaning varies by Mode:
	//   - ModeFull: entire document content
	//   - ModeLines: the single matching line
	//   - ModeSections: the full section content (heading to next heading)
	//   - ModeSnippets: FTS5 snippet with <b></b> highlights
	Text string

	// Before contains context lines preceding the match.
	// Only populated for ModeLines when Options.Context > 0.
	Before []string

	// After contains context lines following the match.
	// Only populated for ModeLines when Options.Context > 0.
	After []string

	// Section is the markdown heading text for the matched section.
	// Only populated for ModeSections. Empty string if the match
	// is in content before the first heading.
	Section string
}
