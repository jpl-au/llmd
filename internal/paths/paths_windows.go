//go:build windows

// Package paths provides platform-specific path handling for llmd.
// See paths_unix.go for full package documentation.
package paths

import (
	"os"
	"path/filepath"
)

// GlobalDir returns the global llmd directory (%APPDATA%\llmd on Windows).
// Creates the directory if it doesn't exist.
func GlobalDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		// Fallback to user home if APPDATA not set
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = home
	}
	dir := filepath.Join(appData, "llmd")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// CacheDir returns the global cache directory (%LOCALAPPDATA%\llmd\cache on Windows).
// Creates the directory if it doesn't exist.
func CacheDir() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		// Fallback to GlobalDir if LOCALAPPDATA not set
		global, err := GlobalDir()
		if err != nil {
			return "", err
		}
		localAppData = global
	} else {
		localAppData = filepath.Join(localAppData, "llmd")
	}
	dir := filepath.Join(localAppData, "cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
