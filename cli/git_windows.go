//go:build windows

package cli

import "strings"

// gitOutput converts raw git command output to a string.
// On Windows, git may produce CRLF line endings when core.autocrlf
// is enabled. We normalise to LF so callers can split on "\n" safely.
func gitOutput(out []byte) string {
	return strings.ReplaceAll(string(out), "\r\n", "\n")
}
