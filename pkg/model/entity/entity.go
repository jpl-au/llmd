// Package entity provides the Entity model for metadata and state.
package entity

// Entity represents metadata, state, or relationships stored in the entities table.
type Entity struct {
	ID        int64
	Key       string
	Namespace string
	Path      string
	Value     string
	Author    string
	Source    string
	CreatedAt int64
	DeletedAt *int64
}
