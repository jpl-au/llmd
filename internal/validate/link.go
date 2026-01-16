// link.go implements link validation for document relationships.
//
// Separated because links have compound validation - both endpoints must
// be valid paths, and the relationship itself has rules (no self-links).
//
// Design: Self-referential links (from == to) are rejected because they
// create meaningless cycles and complicate graph traversal algorithms.
// Path length is NOT validated here - that's done by store.Link() because
// read-only link queries don't need length limits.

package validate

import "fmt"

// Link validates link source and target paths.
//
// Validation rules:
//   - Both paths must be valid (delegates to Path with maxLen=0)
//   - Self-referential links rejected (from == to creates cycles/confusion)
//
// Note: Path length validation uses maxLen=0 here because the actual MaxPath
// is enforced by the store.Link() method via LinkOptions.MaxPath. This function
// only checks structural validity.
func Link(from, to string) error {
	if _, err := Path(from, 0); err != nil {
		return fmt.Errorf("%w: invalid from path: %w", ErrInvalidLink, err)
	}
	if _, err := Path(to, 0); err != nil {
		return fmt.Errorf("%w: invalid to path: %w", ErrInvalidLink, err)
	}
	if from == to {
		return fmt.Errorf("%w: self-referential link", ErrInvalidLink)
	}
	return nil
}
