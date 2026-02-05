package search

// Mode controls the granularity of search results.
// Different modes return different amounts of content per match,
// from just paths to full document content.
type Mode int

const (
	// ModeFull returns the entire document content for each match.
	// Each Result has one Match with the full content in Text.
	// Best for: displaying complete documents, small result sets.
	ModeFull Mode = iota

	// ModeSections returns markdown sections containing matches.
	// Parses the document as markdown and returns only sections (delimited
	// by headings) that contain the search terms. Each Match includes the
	// section header name and the section content.
	// Best for: navigating structured documentation.
	ModeSections

	// ModeLines returns individual matching lines with optional context.
	// Each Match includes the line number, column position, the matching
	// line text, and configurable before/after context lines.
	// Use Options.Context to set the number of context lines (default 0).
	// Best for: grep-like output, code search.
	ModeLines

	// ModePaths returns only document paths without content.
	// Results have empty Matches slices. This is the fastest mode
	// as it skips content retrieval entirely.
	// Best for: listing matching files, counting results.
	ModePaths

	// ModeSnippets uses FTS5's native snippet() function to generate
	// highlighted excerpts around matches. Snippets include <b></b>
	// markers around matched terms and "..." for omitted content.
	// Best for: search result previews, UI display.
	ModeSnippets
)

// Options configures a full-text search operation.
type Options struct {
	// Path limits results to documents under this path prefix.
	// Example: "docs/" matches "docs/readme" and "docs/api/users".
	// Empty string matches all paths.
	Path string

	// Limit caps the number of results returned.
	// Zero means no limit. Results are ordered by FTS5 rank (relevance).
	Limit int

	// Mode controls result granularity. See Mode constants for details.
	// Default is ModeFull.
	Mode Mode

	// Context sets the number of lines before and after each match
	// to include. Only used with ModeLines; ignored for other modes.
	// Zero means no context lines.
	Context int
}
