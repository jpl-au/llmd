package core

import "errors"

// Validation errors for Origin fields. Origin tracks who created a
// record (Author) and where it came from (Source). Both are required
// for all write operations — see Origin.Validate().
var (
	ErrAuthorRequired = errors.New("author is required")
	ErrSourceRequired = errors.New("source is required")
)
