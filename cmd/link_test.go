package cmd

import (
	"regexp"
	"strings"
	"testing"
)

func TestLink(t *testing.T) {
	t.Run("create link", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1 content", "write", "docs/one")
		env.runStdin("doc 2 content", "write", "docs/two")

		out := env.run("link", "docs/one", "docs/two")
		// Output format: "id  docs/one -> docs/two"
		env.contains(out, "docs/one")
		env.contains(out, "docs/two")
		env.contains(out, "->")
	})

	t.Run("create tagged link", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")

		out := env.run("link", "--tag", "depends-on", "docs/one", "docs/two")
		env.contains(out, "depends-on")
	})

	t.Run("list links shows ID", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.run("link", "docs/one", "docs/two")

		out := env.run("link", "--list", "docs/one")
		env.contains(out, "docs/two")
		// ID should be 8 chars at start of line
		if !regexp.MustCompile(`^[a-z0-9]{8}\s`).MatchString(out) {
			t.Errorf("expected ID at start of output, got: %s", out)
		}
	})

	t.Run("list links with tag filter", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.runStdin("doc 3", "write", "docs/three")
		env.run("link", "--tag", "related", "docs/one", "docs/two")
		env.run("link", "--tag", "depends", "docs/one", "docs/three")

		out := env.run("link", "--list", "--tag", "related", "docs/one")
		env.contains(out, "docs/two")
		if strings.Contains(out, "docs/three") {
			t.Error("should not contain docs/three (different tag)")
		}
	})

	t.Run("list orphans", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.runStdin("doc 3", "write", "docs/orphan")
		env.run("link", "docs/one", "docs/two")

		out := env.run("link", "--orphan")
		env.contains(out, "docs/orphan")
		if strings.Contains(out, "docs/one") {
			t.Error("docs/one should not be orphan")
		}
	})

	t.Run("unlink by ID", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")

		// Create link and capture ID from output
		out := env.run("link", "docs/one", "docs/two")
		// Extract ID (first 8 chars)
		id := strings.Fields(out)[0]

		out = env.run("unlink", id)
		env.contains(out, "unlinked")
		env.contains(out, id)

		// Should be orphans now
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

	t.Run("link multiple targets", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.runStdin("doc 3", "write", "docs/three")

		out := env.run("link", "docs/one", "docs/two", "docs/three")
		env.contains(out, "docs/two")
		env.contains(out, "docs/three")

		out = env.run("link", "--list", "docs/one")
		env.contains(out, "docs/two")
		env.contains(out, "docs/three")
	})

	t.Run("JSON output includes ID", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.run("link", "docs/one", "docs/two")

		out := env.run("link", "--list", "docs/one", "-o", "json")
		env.contains(out, `"id"`)
		env.contains(out, `"from_path"`)
		env.contains(out, `"to_path"`)
	})

	t.Run("link by key", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")

		// Get keys from cat -o json
		out := env.run("cat", "docs/one", "-o", "json")
		keyStart := strings.Index(out, `"key":"`) + 7
		key1 := out[keyStart : keyStart+8]

		out = env.run("cat", "docs/two", "-o", "json")
		keyStart = strings.Index(out, `"key":"`) + 7
		key2 := out[keyStart : keyStart+8]

		// Link using keys
		out = env.run("link", key1, key2)
		env.contains(out, "docs/one")
		env.contains(out, "docs/two")
		env.contains(out, "->")

		// Verify link exists
		out = env.run("link", "--list", "docs/one")
		env.contains(out, "docs/two")
	})

	t.Run("link mixed keys and paths", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.runStdin("doc 3", "write", "docs/three")

		// Get key for docs/one
		out := env.run("cat", "docs/one", "-o", "json")
		keyStart := strings.Index(out, `"key":"`) + 7
		key1 := out[keyStart : keyStart+8]

		// Link key to paths
		out = env.run("link", key1, "docs/two", "docs/three")
		env.contains(out, "docs/one")
		env.contains(out, "docs/two")
		env.contains(out, "docs/three")

		// Verify links exist
		out = env.run("link", "--list", "docs/one")
		env.contains(out, "docs/two")
		env.contains(out, "docs/three")
	})

	t.Run("list links by key", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("doc 1", "write", "docs/one")
		env.runStdin("doc 2", "write", "docs/two")
		env.run("link", "docs/one", "docs/two")

		// Get key for docs/one
		out := env.run("cat", "docs/one", "-o", "json")
		keyStart := strings.Index(out, `"key":"`) + 7
		key1 := out[keyStart : keyStart+8]

		// List links using key
		out = env.run("link", "--list", key1)
		env.contains(out, "docs/two")
	})
}
