// Package validate provides input validation for the store layer.
//
// Design Decision: Validation happens at the store layer (not just service layer)
// because the store is the persistence boundary. Anyone with direct store access
// (extensions, tests, future code paths) must have their inputs validated.
// The service layer passes config (MaxPath, MaxContent) via options structs.
package validate

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/internal/path"
)

// Path validates a document path and returns the normalised form.
//
// Validation rules:
//   - Empty paths rejected (would create ambiguous root documents)
//   - Null bytes rejected (security: prevents path injection attacks)
//   - Max length enforced if maxLen > 0 (0 means no limit, used by read operations)
//   - Path normalisation via path.Normalise (handles traversal, leading slashes, etc.)
func Path(p string, maxLen int) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidPath)
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("%w: null byte in path", ErrInvalidPath)
	}
	if maxLen > 0 && len(p) > maxLen {
		return "", ErrPathTooLong
	}

	norm, err := path.Normalise(p)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidPath, err)
	}
	return norm, nil
}
