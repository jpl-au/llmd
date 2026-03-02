//go:build windows

package line

import "strings"

// Native converts LF to CRLF on Windows. Store content is normalised
// to LF on ingest; this restores native line endings when exporting
// back to the filesystem.
func Native(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}
