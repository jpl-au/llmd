package document

// Resolved indicates how a document was found during retrieval.
//
// When retrieving a document, the lookup can happen through different
// mechanisms. Resolved tells the caller which method succeeded, useful
// for debugging, logging, and understanding the resolution path.
type Resolved int

const (
	// ResolvedPath indicates the document was found by its llmd path.
	// Example: Read(ctx, "docs/readme") found a document at that path.
	ResolvedPath Resolved = iota + 1

	// ResolvedKey indicates the document was found by its llmd key.
	// Example: Read(ctx, "abc123def") matched a document's Key field.
	ResolvedKey

	// ResolvedFile indicates the document was found on the filesystem.
	// Used when llmd operates with filesystem fallback enabled.
	ResolvedFile
)
