//go:build !windows

package errlog

import (
	"os"
	"path/filepath"
)

// logDir returns ~/.llmd/logs on Unix.
func logDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".llmd", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
