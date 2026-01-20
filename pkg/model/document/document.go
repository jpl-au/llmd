// Package document provides the Document model for versioned content.
package document

// Document represents a versioned content item stored in the content table.
type Document struct {
	ID        int64
	Key       string
	Namespace string
	Path      string
	Content   string
	Version   int
	Hash      string
	Author    string
	Message   string
	Source    string
	MIME      string
	Meta      *Meta
	CreatedAt int64
	DeletedAt *int64
	Resolved  Resolved
}
