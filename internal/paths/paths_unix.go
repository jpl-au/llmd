//go:build !windows

// Package paths provides platform-specific path handling for llmd.
//
// This package abstracts the differences between Unix and Windows file system
// conventions. On Unix, llmd stores global data in ~/.llmd/. On Windows, it
// uses %APPDATA%\llmd for config and %LOCALAPPDATA%\llmd for cache.
//
// All functions create directories if they don't exist.
package paths

import (
	"os"
	"path/filepath"
)

// GlobalDir returns the global llmd directory (~/.llmd on Unix).
// Creates the directory if it doesn't exist.
func GlobalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".llmd")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// CacheDir returns the global cache directory (~/.llmd/cache on Unix).
// Creates the directory if it doesn't exist.
func CacheDir() (string, error) {
	global, err := GlobalDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(global, "cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
