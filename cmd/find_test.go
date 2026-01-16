package cmd

import (
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	t.Run("basic search", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("This document talks about authentication and JWT tokens.", "write", "docs/auth")
		env.runStdin("This is about user management.", "write", "docs/users")

		out := env.run("find", "authentication")
		env.contains(out, "docs/auth")
	})

	t.Run("no match", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Some content here", "write", "docs/test")

		out := env.run("find", "nonexistent_term_xyz")
		if strings.Contains(out, "docs/test") {
			t.Error("Find(nonexistent) matched, want no match")
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Error handling is important", "write", "docs/errors")
		env.runStdin("More about error codes", "write", "docs/codes")
		env.runStdin("No errors here", "write", "docs/success")

		out := env.run("find", "error")
		env.contains(out, "docs/errors")
		env.contains(out, "docs/codes")
	})

	t.Run("prefix match", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Configure authentication settings", "write", "docs/config")
		env.runStdin("Authorise users properly", "write", "docs/auth")

		out := env.run("find", "auth*")
		env.contains(out, "docs/config")
		env.contains(out, "docs/auth")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Search for this content", "write", "docs/searchable")

		out := env.run("find", "search", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, "docs/searchable")
	})
}

func TestFind_PathScope(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("API authentication required", "write", "docs/api/auth")
	env.runStdin("User authentication flow", "write", "notes/auth")

	out := env.run("find", "authentication", "-p", "docs/")
	env.contains(out, "docs/api/auth")
	if strings.Contains(out, "notes/auth") {
		t.Error("Find(-p docs/) matched notes, want excluded")
	}
}

func TestFind_PathsOnly(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")
	env.runStdin("Document about llmd usage and workflow", "write", "docs/usage")

	out := env.run("find", "llmd", "-l")
	env.contains(out, "docs/guide")
	env.contains(out, "docs/usage")

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			t.Errorf("Find(-l) contains content snippet: %s", line)
		}
	}
}

func TestFind_Deleted(t *testing.T) {
	t.Run("normal excludes deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Important secret content", "write", "docs/secret")
		env.run("rm", "docs/secret")

		out := env.run("find", "secret")
		if strings.Contains(out, "docs/secret") {
			t.Error("Find() matched deleted, want excluded")
		}
	})

	t.Run("-D includes only deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Important secret content", "write", "docs/secret")
		env.run("rm", "docs/secret")

		out := env.run("find", "secret", "-D")
		env.contains(out, "docs/secret")
	})

	t.Run("-A includes all", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")
		env.runStdin("Another document with version control info", "write", "docs/other")
		env.run("rm", "docs/other")

		out := env.run("find", "version", "-A")
		env.contains(out, "docs/guide")
		env.contains(out, "docs/other")
	})
}
