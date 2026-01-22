package core

import "errors"

// Common validation errors for Origin.
var (
	ErrAuthorRequired = errors.New("author is required")
	ErrSourceRequired = errors.New("source is required")
)
