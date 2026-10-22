// gitignore.go manages the .llmd/.gitignore file.
//
// The gitignore uses a whitelist approach: everything is ignored by
// default, and specific files are allowed through with ! patterns.
// This means new files (telemetry logs, temp files) are
// automatically excluded without maintaining a growing blocklist.
//
// llmd only manages .llmd/.gitignore - it never touches the project's
// root .gitignore.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// defaultGitignore is written by InitGitignore for new stores.
// Whitelist approach: ignore everything, then allow only the files
// that should be committed (the document database and this gitignore).
// work.db is excluded - it holds operational state (tasks, audits,
// agent activity) that should not be published.
const defaultGitignore = `# Ignore everything by default
*

# Allow the document database and this file
!llmd.db
!.gitignore
`

// StoreDir is the store directory the working-directory variants of
// these helpers operate on.
const StoreDir = ".llmd"

// gitignorePath returns the path of the .gitignore inside a store
// directory.
func gitignorePath(dir string) string {
	return filepath.Join(dir, ".gitignore")
}

// InitGitignoreIn creates <dir>/.gitignore with sensible defaults,
// creating dir if needed. No-op if the file already exists, which
// preserves user edits on subsequent init calls.
func InitGitignoreIn(dir string) error {
	path := gitignorePath(dir)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultGitignore), 0644)
}

// GitRules returns all non-comment, non-blank lines from
// .llmd/.gitignore in the working directory. See GitRulesIn.
func GitRules() ([]string, error) {
	return GitRulesIn(StoreDir)
}

// GitRulesIn returns all non-comment, non-blank lines from
// <dir>/.gitignore. Returns nil (not error) if the file does not exist.
func GitRulesIn(dir string) ([]string, error) {
	f, err := os.Open(gitignorePath(dir))
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

// GitAllow adds a whitelist entry to .llmd/.gitignore in the working
// directory. See GitAllowIn.
func GitAllow(pattern string) error {
	return GitAllowIn(StoreDir, pattern)
}

// GitAllowIn adds a whitelist entry to <dir>/.gitignore so the pattern
// is committed. The ! prefix is added automatically - callers pass
// the bare pattern (e.g. "reports/" not "!reports/").
func GitAllowIn(dir, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}

	entry := "!" + pattern

	rules, err := GitRulesIn(dir)
	if err != nil {
		return err
	}
	if slices.Contains(rules, entry) {
		return nil
	}

	return appendLine(dir, entry)
}

// GitDeny removes a whitelist entry from .llmd/.gitignore in the
// working directory. See GitDenyIn.
func GitDeny(pattern string) error {
	return GitDenyIn(StoreDir, pattern)
}

// GitDenyIn removes a whitelist entry from <dir>/.gitignore so the
// pattern is no longer committed. The ! prefix is added automatically.
func GitDenyIn(dir, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	entry := "!" + pattern
	return removeLine(dir, entry)
}

// appendLine adds a line to the gitignore file in dir.
func appendLine(dir, line string) error {
	path := gitignorePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, line); err != nil {
		return errors.Join(err, f.Close())
	}
	return f.Close()
}

// removeLine removes a line from the gitignore file in dir.
func removeLine(dir, line string) error {
	path := gitignorePath(dir)

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
