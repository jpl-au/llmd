package search

// Options configures search operations.
type Options struct {
	Path  string // Limit to path prefix
	Limit int    // Max results (0 = no limit)
}
