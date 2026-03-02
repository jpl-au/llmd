//go:build !windows

// Package line provides platform-aware line ending conversion.
package line

// Native returns content unchanged on Unix, where LF is the native
// line ending.
func Native(s string) string {
	return s
}
