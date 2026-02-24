package sdk

// LinkStore is the document linking interface. It manages directed
// relationships between documents.
type LinkStore interface {
	// Add creates a directed link from one document to another.
	Add(from, to, label, author string) error

	// Remove removes a link between two documents.
	Remove(from, to, author string) error

	// List returns links for a document. Dir controls direction:
	// "out" (default), "in", or "both".
	List(path, dir string) ([]Link, error)
}

// Link represents a directed relationship between two documents.
type Link struct {
	From  string
	To    string
	Label string
}
