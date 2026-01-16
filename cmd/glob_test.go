package cmd

import (
	"strings"
	"testing"
)

func TestGlob(t *testing.T) {
	t.Run("basic pattern", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api")
		env.runStdin("content", "write", "notes/meeting")

		out := env.run("glob", "docs/*")
		env.contains(out, "docs/readme")
		env.contains(out, "docs/api")
		if strings.Contains(out, "notes/meeting") {
			t.Error("Glob(docs/*) matched notes, want excluded")
		}
	})

	t.Run("recursive pattern", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api/auth")
		env.runStdin("content", "write", "docs/api/users")

		out := env.run("glob", "docs/**")
		env.contains(out, "docs/readme")
		env.contains(out, "docs/api/auth")
		env.contains(out, "docs/api/users")
	})

	t.Run("no match", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		out := env.run("glob", "nonexistent/*")
		if strings.TrimSpace(out) != "" && strings.Contains(out, "docs") {
			t.Error("Glob(nonexistent/*) matched, want empty")
		}
	})

	t.Run("exact match", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api")

		out := env.run("glob", "docs/readme")
		env.contains(out, "docs/readme")
		if strings.Contains(out, "docs/api") {
			t.Error("Glob(exact) matched other files, want excluded")
		}
	})

	t.Run("question mark wildcard", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/v1")
		env.runStdin("content", "write", "docs/v2")
		env.runStdin("content", "write", "docs/v10")

		out := env.run("glob", "docs/v?")
		env.contains(out, "docs/v1")
		env.contains(out, "docs/v2")
		if strings.Contains(out, "docs/v10") {
			t.Error("Glob(v?) matched v10, want single char only")
		}
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		out := env.run("glob", "docs/*", "-o", "json")
		env.contains(out, "docs/readme")
	})
}

func TestGlob_Deleted(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/active")
	env.runStdin("content", "write", "docs/deleted")
	env.run("rm", "docs/deleted")

	out := env.run("glob", "docs/*")
	env.contains(out, "docs/active")
	if strings.Contains(out, "docs/deleted") {
		t.Error("Glob() matched deleted, want excluded")
	}
}
