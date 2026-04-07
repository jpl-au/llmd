// Package resolve translates identifiers into paths. An identifier
// can be a path, a 9-character key, or either with a :version suffix.
// This logic is shared across all domains (documents, tasks, entities,
// agents, audits) so that keys work uniformly as an alternative to
// paths throughout the platform.
package resolve

import (
	"context"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/key"
)

// KeyToPath translates a key to its path. Each domain provides its own
// implementation backed by a query on its table's key column.
type KeyToPath func(ctx context.Context, k string) (string, error)

// Result holds the resolved path and optional version.
type Result struct {
	Path    string // Resolved path (translated from key if applicable)
	Version *int   // Version from :N suffix, nil if absent
	ByKey   bool   // True when the identifier was resolved via key lookup
}

// Identifier resolves a value that may be a path, a key, or either
// with a :version suffix. When the base value is a valid 9-character
// key the lookup function is called to translate it to a path. If
// lookup is nil or returns an error the value is returned as-is,
// allowing it to fall through to path-based resolution.
func Identifier(ctx context.Context, value string, lookup KeyToPath) Result {
	base, version := ParseVersion(value)

	if key.Validate(base) == nil && lookup != nil {
		if path, err := lookup(ctx, base); err == nil {
			return Result{Path: path, Version: version, ByKey: true}
		}
	}

	return Result{Path: base, Version: version}
}

// ParseVersion splits "value:N" into the base value and an optional
// version pointer. The split is on the last colon so that values
// containing colons (e.g. paths) are handled correctly.
func ParseVersion(value string) (string, *int) {
	idx := strings.LastIndex(value, ":")
	if idx == -1 {
		return value, nil
	}
	suffix := value[idx+1:]
	v, err := strconv.Atoi(suffix)
	if err != nil {
		return value, nil
	}
	return value[:idx], &v
}
