// Package link provides the Link model for document relationships.
package link

// Link represents a relationship between two documents.
type Link struct {
	ID        int64
	Key       string
	From      string
	To        string
	Label     string
	Author    string
	Source    string
	CreatedAt int64
}
