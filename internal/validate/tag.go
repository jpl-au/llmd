// tag.go implements tag string validation.
//
// Separated from path.go because tags have different validation rules -
// they're labels, not hierarchical identifiers. Tags don't need path
// normalisation or traversal protection.
//
// Design: Minimal validation by design. Tags are user-defined labels;
// overly restrictive rules would limit legitimate use cases. Only
// clearly dangerous inputs (empty, null bytes) are rejected.

package validate

import (
	"fmt"
	"strings"
)

// Tag validates a tag string.
//
// Validation rules:
//   - Empty tags rejected (meaningless label)
//   - Null bytes rejected (security: prevents injection in queries/storage)
//
// Note: No max length enforced - tags are typically short labels and SQL handles
// arbitrary lengths. If abuse becomes an issue, add TagOptions with MaxLen.
func Tag(t string) error {
	if t == "" {
		return fmt.Errorf("%w: empty tag", ErrInvalidTag)
	}
	if strings.ContainsRune(t, 0) {
		return fmt.Errorf("%w: null byte in tag", ErrInvalidTag)
	}
	return nil
}
