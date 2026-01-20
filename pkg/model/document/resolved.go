package document

// Resolved indicates how a document was found during retrieval.
type Resolved int

const (
	ResolvedPath Resolved = iota + 1 // found by llmd path
	ResolvedKey                      // found by llmd key
	ResolvedFile                     // found on filesystem
)
