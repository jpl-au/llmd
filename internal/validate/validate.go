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

// LoadLimits converts config limit values into validation thresholds,
// using the default for a value that is missing or not positive (a
// limit of 0 would refuse every path or document).
func LoadLimits(cfg config.Config) Limits {
	return Limits{
		MaxPathLen:    cfg.CountSetting("limits.path_length", 1),
		MaxContentLen: cfg.CountSetting("limits.content_size", 1),
	}
}

// Null rejects strings containing null bytes. Null bytes corrupt
// SQLite text columns and are never valid in document paths, content,
// tag names, or labels.
func Null(s, label string) error {
	if strings.ContainsRune(s, 0) {
		return sdk.Errorf(sdk.ErrInvalidArg, "the %s contains a null byte.", label)
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
		return sdk.Errorf(sdk.ErrInvalidArg, "the path is longer than %d bytes.", lim.MaxPathLen)
	}
	return nil
}

// Content validates document or task body content: rejects null
// bytes and enforces the configured maximum size.
func Content(data []byte, lim Limits) error {
	if len(data) > lim.MaxContentLen {
		return sdk.Errorf(sdk.ErrInvalidArg, "the content is larger than %d bytes.", lim.MaxContentLen)
	}
	if slices.Contains(data, 0) {
		return sdk.Errorf(sdk.ErrInvalidArg, "the content contains a null byte.")
	}
	return nil
}

// Text rejects null bytes in short text fields (tag names, link
// labels, task titles, column names).
func Text(s, label string) error {
	return Null(s, label)
}
