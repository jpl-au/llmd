//go:build windows

package git

import "strings"

// output converts raw git command output to a string.
// On Windows, git may produce CRLF line endings when core.autocrlf
// is enabled. We normalise to LF so callers can split on "\n" safely.
func output(out []byte) string {
	return strings.ReplaceAll(string(out), "\r\n", "\n")
}
