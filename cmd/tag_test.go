package cmd

import (
	"strings"
	"testing"
)

func TestTag(t *testing.T) {
	t.Run("add single", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		env.run("tag", "add", "docs/readme", "important")

		out := env.run("tag", "ls", "docs/readme")
		env.contains(out, "important")
	})

	t.Run("add multiple", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		env.run("tag", "add", "docs/readme", "v1")
		env.run("tag", "add", "docs/readme", "stable")
		env.run("tag", "add", "docs/readme", "reviewed")

		out := env.run("tag", "ls", "docs/readme")
		env.contains(out, "v1")
		env.contains(out, "stable")
		env.contains(out, "reviewed")
	})

	t.Run("add duplicate is idempotent", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		env.run("tag", "add", "docs/readme", "v1")
		env.run("tag", "add", "docs/readme", "v1")

		out := env.run("tag", "list", "docs/readme")
		count := strings.Count(out, "v1")
		if count > 1 {
			t.Errorf("Tag(duplicate) count = %d, want 1", count)
		}
	})
}

func TestTag_Remove(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")
	env.run("tag", "add", "docs/readme", "draft")
	env.run("tag", "add", "docs/readme", "wip")

	env.run("tag", "rm", "docs/readme", "draft")

	out := env.run("tag", "ls", "docs/readme")
	if strings.Contains(out, "draft") {
		t.Error("Tag(rm) draft still present, want removed")
	}
	env.contains(out, "wip")
}

func TestTag_List(t *testing.T) {
	t.Run("list all", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api")
		env.run("tag", "add", "docs/readme", "stable")
		env.run("tag", "add", "docs/api", "beta")

		out := env.run("tag", "ls")
		env.contains(out, "stable")
		env.contains(out, "beta")
	})

	t.Run("find by tag", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api")
		env.runStdin("content", "write", "docs/guide")
		env.run("tag", "add", "docs/readme", "important")
		env.run("tag", "add", "docs/api", "important")
		env.run("tag", "add", "docs/guide", "draft")

		out := env.run("tag", "ls", "docs/readme")
		env.contains(out, "important")

		out = env.run("tag", "ls", "docs/guide")
		env.contains(out, "draft")
		if strings.Contains(out, "important") {
			t.Error("Tag(ls guide) contains important, want excluded")
		}
	})
}

func TestTag_JSONOutput(t *testing.T) {
	t.Run("ls JSON", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")
		env.run("tag", "add", "docs/guide", "documentation")
		env.run("tag", "add", "docs/guide", "reference")

		out := env.run("tag", "ls", "docs/guide", "-o", "json")
		env.contains(out, `"tags"`)
		env.contains(out, `"path"`)
		env.contains(out, "documentation")
		env.contains(out, "reference")
	})

	t.Run("add JSON", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		out := env.run("tag", "add", "docs/guide", "v1.0", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/guide"`)
		env.contains(out, `"action"`)
		env.contains(out, `"add"`)
		env.contains(out, `"tags"`)
		env.contains(out, "v1.0")

		tags := env.run("tag", "ls", "docs/guide")
		env.contains(tags, "v1.0")
	})

	t.Run("rm JSON", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")
		env.run("tag", "add", "docs/guide", "draft")
		env.run("tag", "add", "docs/guide", "needs-review")
		env.run("tag", "add", "docs/guide", "stable")

		out := env.run("tag", "rm", "docs/guide", "draft", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/guide"`)
		env.contains(out, `"action"`)
		env.contains(out, `"remove"`)
		env.contains(out, `"tags"`)
		env.contains(out, "needs-review")
		env.contains(out, "stable")

		tags := env.run("tag", "ls", "docs/guide")
		if strings.Contains(tags, "draft") {
			t.Error("Tag(rm) draft still present, want removed")
		}
		env.contains(tags, "stable")
	})
}

func TestTag_RemoveNonexistent(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")

	// Removing nonexistent tag should fail or be no-op
	_, _ = env.runErr("tag", "rm", "docs/readme", "nonexistent")
}
