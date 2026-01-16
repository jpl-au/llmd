// errors.go defines sentinel errors for validation failures.
//
// Separated to centralise error definitions. These errors are used with
// errors.Is() for type-safe error checking. Each error represents a
// distinct validation failure category.
//
// Design: Sentinel errors (not error types) because validation failures
// don't carry additional context beyond the category. Detailed messages
// are provided by wrapping these with fmt.Errorf in the validation functions.

package validate

import "errors"

var (
	ErrInvalidPath     = errors.New("invalid path")
	ErrPathTooLong     = errors.New("path too long")
	ErrContentTooLarge = errors.New("content too large")
	ErrInvalidTag      = errors.New("invalid tag")
	ErrInvalidLink     = errors.New("invalid link")
)
