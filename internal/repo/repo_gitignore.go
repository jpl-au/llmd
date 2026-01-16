// repo_gitignore.go manages .gitignore entries for local vs shared databases.
//
// Separated from repo.go to isolate gitignore manipulation logic. LLMD supports
// both local databases (ignored by git, not committed) and shared databases
// (committed to the repository). This file provides the IgnoreDB/UnignoreDB
// functions that maintain the .llmd/.gitignore file when switching between modes.
//
// Design: We preserve existing gitignore content and formatting, only adding
// or removing specific database entries. A header comment marks the local
// database section for clarity.

package repo

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const localDBHeader = "# Local databases (not committed)"

// parseGitignore reads a gitignore file and returns its lines (trimmed).
func parseGitignore(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	// Trim whitespace from each line for consistent matching
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return lines, nil
}

// IgnoreDB adds a database to the gitignore (marks as local).
// If dir is empty, discovers .llmd directory from current working directory.
func IgnoreDB(name, dir string) error {
	if dir == "" {
		var err error
		dir, err = DiscoverDir()
		if err != nil {
			return err
		}
	}

	dbFile := DBFileName(name)
	gitignore := filepath.Join(dir, ".gitignore")

	lines, err := parseGitignore(gitignore)
	if err != nil {
		return err
	}

	// Already ignored? Check exact line match.
	if slices.Contains(lines, dbFile) {
		return nil
	}

	// Read original content to preserve formatting.
	// parseGitignore succeeded above, so file exists and is readable.
	content, err := os.ReadFile(gitignore)
	if err != nil {
		return err
	}
	s := string(content)

	// Add header if not present
	if !slices.Contains(lines, localDBHeader) {
		s += "\n" + localDBHeader + "\n"
	}

	// Append database
	s += dbFile + "\n"

	if err := os.WriteFile(gitignore, []byte(s), 0644); err != nil {
		return err
	}
	return nil
}

// UnignoreDB removes a database from the gitignore (marks as shared).
// If dir is empty, discovers .llmd directory from current working directory.
func UnignoreDB(name, dir string) error {
	if dir == "" {
		var err error
		dir, err = DiscoverDir()
		if err != nil {
			return err
		}
	}

	dbFile := DBFileName(name)
	gitignore := filepath.Join(dir, ".gitignore")

	content, err := os.ReadFile(gitignore)
	if err != nil {
		return err
	}

	// Remove the database line, preserving other content
	lines := strings.Split(string(content), "\n")
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) != dbFile {
			out = append(out, line)
		}
	}

	// Clean up: remove header if no local databases remain after it
	result := strings.Join(out, "\n")
	if idx := strings.Index(result, localDBHeader); idx != -1 {
		rest := strings.TrimSpace(result[idx+len(localDBHeader):])
		// If nothing meaningful after header, remove it
		if rest == "" || !strings.Contains(rest, ".db") {
			result = strings.TrimSuffix(result[:idx], "\n")
		}
	}

	if err := os.WriteFile(gitignore, []byte(result), 0644); err != nil {
		return err
	}
	return nil
}

// IsIgnored checks if a database is in the gitignore.
// If dir is empty, discovers .llmd directory from current working directory.
func IsIgnored(name, dir string) (bool, error) {
	if dir == "" {
		var err error
		dir, err = DiscoverDir()
		if err != nil {
			return false, err
		}
	}

	dbFile := DBFileName(name)
	gitignore := filepath.Join(dir, ".gitignore")

	lines, err := parseGitignore(gitignore)
	if err != nil {
		return false, err
	}

	return slices.Contains(lines, dbFile), nil
}
