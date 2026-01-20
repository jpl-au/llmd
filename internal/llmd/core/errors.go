package core

import "errors"

// Common errors for write operations.
var (
	ErrAuthorRequired = errors.New("author is required")
	ErrSourceRequired = errors.New("source is required")
)
