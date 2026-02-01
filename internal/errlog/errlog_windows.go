//go:build windows

package errlog

import (
	"os"
	"path/filepath"
)

// logDir returns %APPDATA%\llmd\logs on Windows.
func logDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = home
	}
	dir := filepath.Join(appData, "llmd", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
