// Package tag provides the Tag model for document tagging.
package tag

// Tag represents a tag attached to a document.
type Tag struct {
	ID        int64
	Key       string
	Path      string
	Tag       string
	Author    string
	Source    string
	CreatedAt int64
}
