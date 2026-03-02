//go:build !windows

package git

// output converts raw git command output to a string.
// On Unix, git always uses LF line endings — no conversion needed.
func output(out []byte) string {
	return string(out)
}
