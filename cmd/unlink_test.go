package cmd

import (
	"strings"
	"testing"
)

func TestUnlink(t *testing.T) {
	t.Run("unlink by ID", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")

		// Create link and capture ID
		out := env.run("link", "docs/one", "docs/two")
		id := strings.Fields(out)[0]

		out = env.run("unlink", id)
		env.contains(out, "unlinked")
		env.contains(out, id)

		// Verify link is gone
		out = env.run("link", "--orphan")
		env.contains(out, "docs/one")
		env.contains(out, "docs/two")
	})

	t.Run("unlink by tag", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.runStdin("doc 3", "write", "docs/three")
		env.run("link", "--tag", "temp", "docs/one", "docs/two")
		env.run("link", "--tag", "temp", "docs/one", "docs/three")

		out := env.run("unlink", "--tag", "temp")
		env.contains(out, "unlinked")
		env.contains(out, "2") // 2 links removed

		// All should be orphans now
		out = env.run("link", "--orphan")
		env.contains(out, "docs/one")
		env.contains(out, "docs/two")
		env.contains(out, "docs/three")
	})

	t.Run("unlink nonexistent ID", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("unlink", "notfound")
		if err == nil {
			t.Fatal("expected error for nonexistent ID")
		}
	})

	t.Run("unlink requires ID or tag", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("unlink")
		if err == nil {
			t.Fatal("expected error when no ID or tag provided")
		}
	})

	t.Run("unlink rejects multiple args", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("unlink", "id1", "id2")
		if err == nil {
			t.Fatal("expected error for multiple args")
		}
	})

	t.Run("unlink tag with no matches", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.run("link", "--tag", "keep", "docs/one", "docs/two")

		// Unlink a different tag
		out := env.run("unlink", "--tag", "nonexistent")
		env.contains(out, "unlinked")
		env.contains(out, "0") // 0 links removed

		// Original link should still exist
		out = env.run("link", "--list", "docs/one")
		env.contains(out, "docs/two")
	})

	t.Run("unlink JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")

		// Create link and capture ID
		out := env.run("link", "docs/one", "docs/two")
		id := strings.Fields(out)[0]

		out = env.run("unlink", id, "-o", "json")
		env.contains(out, `"id"`)
		env.contains(out, id)
	})

	t.Run("unlink tag JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.run("link", "--tag", "temp", "docs/one", "docs/two")

		out := env.run("unlink", "--tag", "temp", "-o", "json")
		env.contains(out, `"tag"`)
		env.contains(out, `"count"`)
		env.contains(out, `"temp"`)
	})
}
