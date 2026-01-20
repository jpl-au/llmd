package search

// Options configures search operations.
type Options struct {
	Path        string     // Limit to path prefix
	Limit       int        // Max results (0 = no limit) - used by FullText
	IgnoreCase  bool       // Case-insensitive matching - used by Regex
	InvertMatch bool       // Show non-matching - used by Regex
	Mode        ResultMode // What to return - used by Regex
	Context     int        // Lines of context - used by Regex
}

// ResultMode determines what regex search returns.
type ResultMode int

const (
	ModeContent ResultMode = iota + 1 // Return matching lines with context
	ModeFiles                         // Return only file paths
	ModeCount                         // Return match counts per file
)
