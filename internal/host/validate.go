// validate.go provides input validation at the API boundary.
//
// Every string entering the system (paths, content, tags, labels) is
// checked here before reaching the store. Null bytes are always
// rejected. Path length and content size limits are configurable via
// the config file; sensible defaults apply when no config is set.

package host

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

const (
	defaultMaxPathLen    = 1024
	defaultMaxContentLen = 10 * 1024 * 1024 // 10MB
)

// limits holds configurable validation thresholds loaded from config.
type limits struct {
	MaxPathLen    int
	MaxContentLen int
}

// loadLimits reads validation limits from config, falling back to
// defaults for missing or unparseable values.
func loadLimits(cfg map[string]string) limits {
	return limits{
		MaxPathLen:    config.Int(cfg, "max-path-length", defaultMaxPathLen),
		MaxContentLen: config.Int(cfg, "max-content-size", defaultMaxContentLen),
	}
}

// checkNull rejects strings containing null bytes. Null bytes corrupt
// SQLite text columns and are never valid in document paths, content,
// tag names, or labels.
func checkNull(s, label string) error {
	if strings.ContainsRune(s, 0) {
		return fmt.Errorf("%w: %s contains null byte", sdk.ErrInvalidArg, label)
	}
	return nil
}

// checkPath validates a document path: rejects null bytes and enforces
// the configured maximum length.
func checkPath(path string, lim limits) error {
	if err := checkNull(path, "path"); err != nil {
		return err
	}
	if len(path) > lim.MaxPathLen {
		return fmt.Errorf("%w: path exceeds %d bytes", sdk.ErrInvalidArg, lim.MaxPathLen)
	}
	return nil
}

// checkContent validates document or task body content: rejects null
// bytes and enforces the configured maximum size.
func checkContent(data []byte, lim limits) error {
	if len(data) > lim.MaxContentLen {
		return fmt.Errorf("%w: content exceeds %d bytes", sdk.ErrInvalidArg, lim.MaxContentLen)
	}
	if slices.Contains(data, 0) {
		return fmt.Errorf("%w: content contains null byte", sdk.ErrInvalidArg)
	}
	return nil
}

// checkText rejects null bytes in short text fields (tag names, link
// labels, task titles, column names).
func checkText(s, label string) error {
	return checkNull(s, label)
}
