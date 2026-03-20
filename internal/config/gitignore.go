// gitignore.go manages the .llmd/.gitignore file.
//
// The gitignore uses a whitelist approach: everything is ignored by
// default, and specific files are allowed through with ! patterns.
// This means new files (telemetry logs, temp files, plugins) are
// automatically excluded without maintaining a growing blocklist.
//
// llmd only manages .llmd/.gitignore - it never touches the project's
// root .gitignore.
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
// Whitelist approach: ignore everything, then allow only the files
// that should be committed (the database and this gitignore).
const defaultGitignore = `# Ignore everything by default
*

# Allow the database and this file
!*.db
!.gitignore
`

// gitignorePath returns the path to the local .llmd/.gitignore.
func gitignorePath() string {
	return filepath.Join(".llmd", ".gitignore")
}

// InitGitignore creates .llmd/.gitignore with sensible defaults.
// No-op if the file already exists - preserves user edits on
// subsequent init calls.
func InitGitignore() error {
	path := gitignorePath()
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(defaultGitignore), 0644)
}

// GitRules returns all non-comment, non-blank lines from
// .llmd/.gitignore. Returns nil (not error) if the file does not exist.
func GitRules() ([]string, error) {
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

// GitAllow adds a whitelist entry to .llmd/.gitignore so the pattern
// is committed. The ! prefix is added automatically - callers pass
// the bare pattern (e.g. "reports/" not "!reports/").
func GitAllow(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}

	entry := "!" + pattern

	rules, err := GitRules()
	if err != nil {
		return err
	}
	if slices.Contains(rules, entry) {
		return nil
	}

	return appendLine(entry)
}

// GitDeny removes a whitelist entry from .llmd/.gitignore so the
// pattern is no longer committed. The ! prefix is added automatically.
func GitDeny(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	entry := "!" + pattern
	return removeLine(entry)
}

// appendLine adds a line to the gitignore file.
func appendLine(line string) error {
	path := gitignorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, line)
	return err
}

// removeLine removes a line from the gitignore file.
func removeLine(line string) error {
	path := gitignorePath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("pattern not found: %s", line)
	}
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) == line {
			found = true
			continue
		}
		out = append(out, l)
	}
	if !found {
		return fmt.Errorf("pattern not found: %s", line)
	}

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
}

// EnsureAllowed adds a whitelist entry if not already present.
// Used by mirror to ensure its output directory is committed.
func EnsureAllowed(pattern string) error {
	return GitAllow(pattern)
}
