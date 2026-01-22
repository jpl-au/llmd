// Package core provides shared types embedded across llmd models.
package core

// Origin tracks the origin of data: who created it and from where.
//
// Embed this struct in models that record authorship and source.
// CreatedAt is handled automatically at the database INSERT layer,
// not included here to avoid duplication.
//
// Example embedding:
//
//	type Tag struct {
//	    Key       string
//	    Relation  string
//	    Value     Value
//	    core.Origin
//	    CreatedAt int64
//	}
type Origin struct {
	// Author identifies who created this record.
	// Typically a username, email, or "system" for automated operations.
	// Required for all write operations.
	Author string

	// Source identifies where this record originated.
	// Standard values: "cli", "mcp", "api", "import", "sync".
	// Helps track how data entered the system.
	Source string

	// Message is an optional description of the change.
	// Similar to a git commit message.
	Message string
}

// Validate checks that required fields are set.
func (o Origin) Validate() error {
	if o.Author == "" {
		return ErrAuthorRequired
	}
	if o.Source == "" {
		return ErrSourceRequired
	}
	return nil
}
