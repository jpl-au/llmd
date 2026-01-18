package cmd

import (
	"strings"
	"testing"
)

func TestVacuum(t *testing.T) {
	t.Run("basic vacuum", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.run("rm", "docs/readme")

		out := env.run("ls", "-R", "-D")
		env.contains(out, "docs/readme")

		env.run("vacuum", "--force")

		out = env.run("ls", "-R", "-D")
		if strings.Contains(out, "docs/readme") {
			t.Error("Vacuum() doc still in deleted, want permanently removed")
		}
	})

	t.Run("preserves active", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("active content", "write", "docs/active")
		env.runStdin("deleted content", "write", "docs/deleted")
		env.run("rm", "docs/deleted")

		env.run("vacuum", "--force")

		out := env.run("ls", "-R")
		env.contains(out, "docs/active")

		out = env.run("ls", "-R", "-D")
		if strings.Contains(out, "docs/deleted") {
			t.Error("Vacuum() deleted doc still present, want removed")
		}
	})

	t.Run("no deleted docs", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		_ = env.run("vacuum", "--force")

		out := env.run("cat", "docs/readme")
		env.equals(out, "content")
	})

	t.Run("multiple deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/a")
		env.runStdin("content", "write", "docs/b")
		env.runStdin("content", "write", "docs/c")
		env.run("rm", "docs/a")
		env.run("rm", "docs/b")

		env.run("vacuum", "--force")

		out := env.run("ls", "-R", "-A")
		env.contains(out, "docs/c")
		if strings.Contains(out, "docs/a") || strings.Contains(out, "docs/b") {
			t.Error("Vacuum() some deleted docs still present, want all removed")
		}
	})
}

func TestVacuum_DryRun(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")
	env.run("rm", "docs/readme")

	env.run("vacuum", "--dry-run")

	out := env.run("ls", "-R", "-D")
	env.contains(out, "docs/readme")
}

func TestVacuum_PreservesHistory(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/active")
	env.runStdin("v2", "write", "docs/active")
	env.runStdin("v3", "write", "docs/active")

	env.runStdin("deleted", "write", "docs/deleted")
	env.run("rm", "docs/deleted")
	env.run("vacuum", "--force")

	out := env.run("history", "docs/active")
	env.contains(out, "v1")
	env.contains(out, "v2")
	env.contains(out, "v3")
}

func TestVacuum_OlderThan(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")
	env.runStdin("API content", "write", "docs/api")
	env.run("rm", "docs/guide")
	env.run("rm", "docs/api")

	out := env.run("ls", "-R", "-D")
	env.contains(out, "docs/guide")
	env.contains(out, "docs/api")

	// Just deleted, so --older-than 1d should not delete
	env.run("vacuum", "--force", "--older-than", "1d")

	out = env.run("ls", "-R", "-D")
	env.contains(out, "docs/guide")
	env.contains(out, "docs/api")
}

func TestVacuum_FTSCleanup(t *testing.T) {
	// Verifies that vacuum removes documents from the FTS index.
	// Soft-deleted docs remain searchable with -D until vacuum permanently deletes them.
	env := newTestEnv(t)
	env.runStdin("searchable unique content here", "write", "docs/fts-test")

	// Verify searchable when active
	out := env.run("find", "searchable")
	env.contains(out, "docs/fts-test")

	// Soft-delete - should still be findable with -D
	env.run("rm", "docs/fts-test")
	out = env.run("find", "searchable", "-D")
	env.contains(out, "docs/fts-test")

	// After vacuum, should not be findable even with -A
	env.run("vacuum", "--force")
	out = env.run("find", "searchable", "-A")
	if strings.Contains(out, "docs/fts-test") {
		t.Error("Vacuum() doc still in FTS index, want removed")
	}
}

func TestVacuum_Path(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")
	env.runStdin("Notes content", "write", "notes/meeting")
	env.run("rm", "docs/guide")
	env.run("rm", "notes/meeting")

	out := env.run("ls", "-R", "-D")
	env.contains(out, "docs/guide")
	env.contains(out, "notes/meeting")

	env.run("vacuum", "--force", "-p", "docs/")

	out = env.run("ls", "-R", "-D")
	if strings.Contains(out, "docs/guide") {
		t.Error("Vacuum(-p docs/) did not remove docs/guide")
	}
	env.contains(out, "notes/meeting")
}
