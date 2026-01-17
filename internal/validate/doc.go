// Package validate provides input validation for llmd's domain types.
//
// This package enforces security and data integrity rules at the boundary
// between user input and the storage layer. Each validation function returns
// nil on success or a descriptive error on failure.
//
// # Design Philosophy
//
// Validation is minimal by design. We reject clearly dangerous inputs (null
// bytes, path traversal, excessive sizes) but avoid overly restrictive rules
// that would limit legitimate use cases. The goal is security without
// arbitrarily constraining users.
//
// # Validation Functions
//
// Path validates and normalizes document paths with traversal protection.
// Tag validates tag strings (labels, not hierarchical identifiers).
// Link validates relationships between documents.
// Content validates document body size limits.
//
// # Error Handling
//
// All validation errors wrap one of the sentinel errors defined in errors.go
// (ErrInvalidPath, ErrInvalidTag, etc.). Use errors.Is() for type-safe
// error checking:
//
//	if errors.Is(err, validate.ErrInvalidPath) {
//	    // handle invalid path
//	}
package validate
