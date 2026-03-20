// Package path provides document path normalisation and validation utilities.
//
// All document paths in llmd pass through this package before storage or
// retrieval. Validation ensures paths are safe for database storage.
//
// Security: Path traversal attacks are blocked by rejecting any path
// containing "..". Combined with normalisation at the API boundary, this
// provides defence-in-depth against escaping the document store.
//
// Normalisation rules:
//   - Paths use forward slashes (cross-platform)
//   - No leading or trailing slashes
//   - No "." or ".." components
//   - Empty paths are rejected
//   - .md extension is stripped (docs/readme.md becomes docs/readme)
//
// Document paths are DB keys, not filesystem paths. Normalise and Direct
// use the path package (always forward-slash) so behaviour is identical on
// all platforms. Filesystem path construction (ResolveDB, MirrorDir) uses
// filepath, which is platform-aware.
package path

import (
	"errors"
	"fmt"
	slashpath "path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ErrInvalid indicates the provided document path is invalid.
var ErrInvalid = errors.New("invalid document path")

// ErrTooLong indicates the document path exceeds the configured maximum length.
var ErrTooLong = errors.New("document path too long")

// ErrInvalidDB indicates the --db shorthand name is invalid.
var ErrInvalidDB = errors.New("invalid database name")

// ResolveDB converts a --db shorthand name to a full store path.
// An empty string returns the default path (.llmd/llmd.db). A bare
// name like "docs" becomes .llmd/llmd-docs.db. Absolute paths,
// Windows volume names (e.g. C:), and .db suffixes are returned
// unchanged - no sanitisation is applied since the user is providing
// a real path.
//
// Shorthand names are sanitised: spaces become dashes, consecutive
// dashes collapse, and leading/trailing dashes are trimmed. Names
// containing control characters, Windows-illegal characters
// (< > : " | ? *), or path traversal (..) are rejected.
func ResolveDB(name string) (string, error) {
	if name == "" {
		return filepath.Join(".llmd", "llmd.db"), nil
	}

	// Explicit paths are returned as-is.
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" || strings.ContainsAny(name, "/\\") || strings.HasSuffix(name, ".db") {
		return name, nil
	}

	// Shorthand name - sanitise.
	sanitised, err := sanitiseDBName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(".llmd", "llmd-"+sanitised+".db"), nil
}

// sanitiseDBName validates and cleans a bare --db shorthand name.
func sanitiseDBName(name string) (string, error) {
	// Reject control characters (0–31) and null bytes.
	for i := 0; i < len(name); i++ {
		if name[i] <= 0x1f {
			return "", fmt.Errorf("%w: contains control character", ErrInvalidDB)
		}
	}

	// Reject non-UTF-8 sequences.
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("%w: contains invalid UTF-8", ErrInvalidDB)
	}

	// Reject Windows-illegal characters.
	if strings.ContainsAny(name, `<>:"|?*`) {
		return "", fmt.Errorf("%w: contains illegal character", ErrInvalidDB)
	}

	// Reject path traversal.
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("%w: contains path traversal", ErrInvalidDB)
	}

	// Convert spaces to dashes, collapse consecutive dashes, trim.
	s := strings.ReplaceAll(name, " ", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	if s == "" {
		return "", fmt.Errorf("%w: empty after sanitisation", ErrInvalidDB)
	}

	return s, nil
}

// MirrorDir returns the mirror directory for the active database.
// Empty dbPath uses the default store, producing .llmd/llmd/.
// A name like "docs" produces .llmd/llmd-docs/.
func MirrorDir(dbPath string) (string, error) {
	resolved, err := ResolveDB(dbPath)
	if err != nil {
		return "", err
	}
	base := filepath.Base(resolved)
	name := strings.TrimSuffix(base, ".db")
	return filepath.Join(".llmd", name), nil
}

// Normalise cleans and validates a document path.
// It ensures paths use forward slashes, have no leading/trailing slashes,
// and contain no directory traversal sequences.
//
// Document paths are DB keys, not filesystem paths, so this function uses
// the path package (always forward-slash) and behaves identically on all
// platforms. Backslashes in user input are converted to forward slashes so
// that Windows-style paths are accepted on any platform.
func Normalise(p string) (string, error) {
	if p == "" {
		return "", ErrInvalid
	}

	// Convert backslashes to forward slashes. filepath.ToSlash won't do
	// this on Unix (where backslash is a valid filename character), but we
	// want to accept Windows-style input on any platform.
	p = strings.ReplaceAll(p, "\\", "/")

	p = slashpath.Clean(p)

	// Reject absolute paths - document paths must always be relative.
	if slashpath.IsAbs(p) {
		return "", ErrInvalid
	}

	// Remove leading/trailing slashes left after cleaning.
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")

	// Strip .md extension (case-insensitive).
	if len(p) > 3 && strings.EqualFold(p[len(p)-3:], ".md") {
		p = p[:len(p)-3]
	}

	if p == "" || p == "." || p == ".." {
		return "", ErrInvalid
	}

	if strings.Contains(p, "..") {
		return "", ErrInvalid
	}

	return p, nil
}

// Direct reports whether path is a direct child of prefix.
// Both paths should use forward slashes. Backslashes in prefix are
// converted so that Windows-style input is accepted on any platform.
//
// Examples (prefix="docs"):
//   - "docs/readme" -> true (direct child)
//   - "docs/api/auth" -> false (nested)
//   - "docs" -> true (exact match)
//
// Examples (prefix=""):
//   - "readme" -> true (top level)
//   - "docs/readme" -> false (nested)
func Direct(p, prefix string) bool {
	// Convert backslashes and remove trailing slash from prefix.
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	prefix = strings.TrimSuffix(prefix, "/")

	if p == prefix {
		return true
	}

	var remainder string
	if prefix == "" {
		remainder = p
	} else if strings.HasPrefix(p, prefix+"/") {
		remainder = p[len(prefix)+1:]
	} else {
		return false
	}

	return !strings.Contains(remainder, "/")
}
