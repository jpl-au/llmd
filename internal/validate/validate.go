// Package validate provides input validation for data entering the
// system. Every string (paths, content, tags, labels) is checked here
// before reaching the store. Null bytes are always rejected. Path
// length and content size limits are configurable via the config file;
// sensible defaults apply when no config is set.
//
// This package is used at the API boundary (internal/host) but is
// available to any layer that needs to validate input - including
// bulk operations and extensions.
package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

// Limits holds configurable validation thresholds loaded from config.
type Limits struct {
	MaxPathLen    int
	MaxContentLen int
}

// LoadLimits converts config limit values into validation thresholds.
func LoadLimits(lim config.LimitConfig) Limits {
	return Limits{
		MaxPathLen:    lim.PathLength,
		MaxContentLen: lim.ContentSize,
	}
}

// Null rejects strings containing null bytes. Null bytes corrupt
// SQLite text columns and are never valid in document paths, content,
// tag names, or labels.
func Null(s, label string) error {
	if strings.ContainsRune(s, 0) {
		return fmt.Errorf("%w: %s contains null byte", sdk.ErrInvalidArg, label)
	}
	return nil
}

// Path validates a document path: rejects null bytes and enforces
// the configured maximum length.
func Path(path string, lim Limits) error {
	if err := Null(path, "path"); err != nil {
		return err
	}
	if len(path) > lim.MaxPathLen {
		return fmt.Errorf("%w: path exceeds %d bytes", sdk.ErrInvalidArg, lim.MaxPathLen)
	}
	return nil
}

// Content validates document or task body content: rejects null
// bytes and enforces the configured maximum size.
func Content(data []byte, lim Limits) error {
	if len(data) > lim.MaxContentLen {
		return fmt.Errorf("%w: content exceeds %d bytes", sdk.ErrInvalidArg, lim.MaxContentLen)
	}
	if slices.Contains(data, 0) {
		return fmt.Errorf("%w: content contains null byte", sdk.ErrInvalidArg)
	}
	return nil
}

// Text rejects null bytes in short text fields (tag names, link
// labels, task titles, column names).
func Text(s, label string) error {
	return Null(s, label)
}
