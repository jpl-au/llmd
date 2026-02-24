package sdk

// TagStore is the document tagging interface. It manages tags
// attached to documents.
type TagStore interface {
	// Add attaches a tag to a document.
	Add(path, name, author string) error

	// Remove removes a tag from a document.
	Remove(path, name, author string) error

	// List returns all tags on a document.
	List(path string) ([]Tag, error)

	// All returns every tag in the store with usage counts.
	All() ([]TagInfo, error)

	// Find returns document paths that have the given tag.
	Find(name string) ([]string, error)
}

// Tag represents a tag attached to a document.
type Tag struct {
	Name string
	Path string
}

// TagInfo represents a tag with its usage count across documents.
type TagInfo struct {
	Name  string
	Count int
}
