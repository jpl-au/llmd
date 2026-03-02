// Package path provides document path normalisation and validation utilities.
//
// All document paths in llmd pass through this package before storage or
// retrieval. Validation ensures paths are safe for both database storage
// and filesystem mirroring.
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
// Platform-specific handling: The Normalise and Direct functions are
// implemented separately for Windows and Unix systems (see
// path_windows.go, path_unix.go). This ensures correct backslash
// handling on each platform.
package path

import (
	"errors"
	"fmt"
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
// unchanged — no sanitisation is applied since the user is providing
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

	// Shorthand name — sanitise.
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
