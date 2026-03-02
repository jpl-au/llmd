// gitignore.go manages the .llmd/.gitignore file.
//
// The gitignore file controls which files inside .llmd/ are excluded
// from version control. It uses standard gitignore format: one pattern
// per line, # comments, blank lines for readability.
//
// llmd only manages .llmd/.gitignore — it never touches the project's
// root .gitignore. The default file ignores SQLite temp files and the
// mirror output directory for the default database.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// defaultGitignore is written by InitGitignore for new stores.
const defaultGitignore = `# SQLite temp files (always ignore)
*.db-wal
*.db-shm

# Mirrored files (generated from store)
llmd/
`

// gitignorePath returns the path to the local .llmd/.gitignore.
func gitignorePath() string {
	return filepath.Join(".llmd", ".gitignore")
}

// InitGitignore creates .llmd/.gitignore with sensible defaults.
// No-op if the file already exists — preserves user edits on
// subsequent init calls.
func InitGitignore() error {
	path := gitignorePath()
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(defaultGitignore), 0644)
}

// IgnorePatterns returns all non-comment, non-blank lines from
// .llmd/.gitignore. Returns nil (not error) if the file does not exist.
func IgnorePatterns() ([]string, error) {
	f, err := os.Open(gitignorePath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, sc.Err()
}

// AddIgnore appends a pattern to .llmd/.gitignore if not already
// present. Creates the file if it does not exist.
func AddIgnore(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}

	patterns, err := IgnorePatterns()
	if err != nil {
		return err
	}
	if slices.Contains(patterns, pattern) {
		return nil
	}

	path := gitignorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, pattern)
	return err
}

// RemoveIgnore removes a pattern from .llmd/.gitignore. Returns an
// error if the pattern is not found or the file does not exist.
func RemoveIgnore(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	path := gitignorePath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("pattern not found: %s", pattern)
	}
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			found = true
			continue
		}
		out = append(out, line)
	}
	if !found {
		return fmt.Errorf("pattern not found: %s", pattern)
	}

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
}

// EnsureIgnored adds a pattern if not already present.
func EnsureIgnored(pattern string) error {
	return AddIgnore(pattern)
}
