//go:build !windows

package cli

// gitOutput converts raw git command output to a string.
// On Unix, git always uses LF line endings — no conversion needed.
func gitOutput(out []byte) string {
	return string(out)
}
