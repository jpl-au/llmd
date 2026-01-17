package cmd

import (
	"strings"
	"testing"
)

func TestRevert(t *testing.T) {
	t.Run("basic revert by path and version", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("version 1", "write", "docs/readme")
		env.runStdin("version 2", "write", "docs/readme")
		env.runStdin("version 3", "write", "docs/readme")

		// Verify we're at version 3
		out := env.run("cat", "docs/readme")
		env.equals(out, "version 3")

		// Revert to version 1
		out = env.run("revert", "docs/readme", "1")
		env.contains(out, "Reverted docs/readme to v1")
		env.contains(out, "now v4")

		// Verify content is back to version 1
		out = env.run("cat", "docs/readme")
		env.equals(out, "version 1")

		// Verify we now have 4 versions
		out = env.run("history", "docs/readme")
		env.contains(out, "v4")
		env.contains(out, "v3")
		env.contains(out, "v2")
		env.contains(out, "v1")
	})

	t.Run("revert by key", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("version 1", "write", "docs/readme")
		env.runStdin("version 2", "write", "docs/readme")

		// Get the key for version 1 from JSON output
		out := env.run("history", "docs/readme", "-o", "json")
		// The JSON array has version 2 first (newest), then version 1
		// Extract the key for version 1 (the one with "version":1)
		if !strings.Contains(out, `"version":1`) {
			t.Fatalf("history JSON missing version 1: %s", out)
		}

		// Get the key from cat -v 1 -o json
		out = env.run("cat", "docs/readme", "-v", "1", "-o", "json")
		// Extract key from JSON - it's an 8-char string
		// Format: {"key":"abc12345",...}
		keyStart := strings.Index(out, `"key":"`) + 7
		key := out[keyStart : keyStart+8]

		// Revert using the key
		out = env.run("revert", key)
		env.contains(out, "Reverted docs/readme to v1")

		// Verify content
		out = env.run("cat", "docs/readme")
		env.equals(out, "version 1")
	})

	t.Run("revert with custom message", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("original", "write", "docs/readme")
		env.runStdin("changed", "write", "docs/readme")

		env.run("revert", "docs/readme", "1", "-m", "Rolling back bad changes")

		out := env.run("history", "docs/readme")
		env.contains(out, "Rolling back bad changes")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("v1", "write", "docs/readme")
		env.runStdin("v2", "write", "docs/readme")

		out := env.run("revert", "docs/readme", "1", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/readme"`)
		env.contains(out, `"reverted_to":1`)
		env.contains(out, `"new_version":3`)
	})
}

func TestRevert_Errors(t *testing.T) {
	t.Run("version not found", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		_, err := env.runErr("revert", "docs/readme", "99")
		if err == nil {
			t.Error("Revert(nonexistent version) = nil, want error")
		}
	})

	t.Run("path not found", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("revert", "docs/nonexistent", "1")
		if err == nil {
			t.Error("Revert(nonexistent path) = nil, want error")
		}
	})

	t.Run("key not found", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("revert", "zzzzzzzz")
		if err == nil {
			t.Error("Revert(nonexistent key) = nil, want error")
		}
	})

	t.Run("path without version", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		_, err := env.runErr("revert", "docs/readme")
		if err == nil {
			t.Error("Revert(path without version) = nil, want error")
		}
	})

	t.Run("deleted document", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.run("rm", "docs/readme")

		_, err := env.runErr("revert", "docs/readme", "1")
		if err == nil {
			t.Error("Revert(deleted doc) = nil, want error")
		}
	})

	t.Run("invalid version format", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		_, err := env.runErr("revert", "docs/readme", "abc")
		if err == nil {
			t.Error("Revert(invalid version) = nil, want error")
		}
	})
}

func TestRevert_PreservesHistory(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/readme")
	env.runStdin("v2", "write", "docs/readme")
	env.runStdin("v3", "write", "docs/readme")

	// Revert to v1
	env.run("revert", "docs/readme", "1")

	// All original versions should still be accessible
	tests := []struct {
		version string
		want    string
	}{
		{"1", "v1"},
		{"2", "v2"},
		{"3", "v3"},
		{"4", "v1"}, // The revert created v4 with v1's content
	}

	for _, tc := range tests {
		t.Run("v"+tc.version, func(t *testing.T) {
			out := env.run("cat", "-v", tc.version, "docs/readme")
			env.equals(out, tc.want)
		})
	}
}

func TestRevert_MultipleReverts(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/readme")
	env.runStdin("v2", "write", "docs/readme")
	env.runStdin("v3", "write", "docs/readme")

	// Revert to v1 (creates v4)
	env.run("revert", "docs/readme", "1")
	out := env.run("cat", "docs/readme")
	env.equals(out, "v1")

	// Revert to v2 (creates v5)
	env.run("revert", "docs/readme", "2")
	out = env.run("cat", "docs/readme")
	env.equals(out, "v2")

	// Revert to v3 (creates v6)
	env.run("revert", "docs/readme", "3")
	out = env.run("cat", "docs/readme")
	env.equals(out, "v3")

	// Should have 6 versions now
	out = env.run("history", "docs/readme")
	env.contains(out, "v6")
}

func TestRevert_KeyFlag(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("version 1", "write", "docs/readme")
	env.runStdin("version 2", "write", "docs/readme")

	// Get the key for version 1
	out := env.run("cat", "docs/readme", "-v", "1", "-o", "json")
	keyStart := strings.Index(out, `"key":"`) + 7
	key := out[keyStart : keyStart+8]

	// Revert using --key flag
	out = env.run("revert", "--key", key)
	env.contains(out, "Reverted docs/readme to v1")

	// Verify content
	out = env.run("cat", "docs/readme")
	env.equals(out, "version 1")
}

func TestRevert_VersionValidation(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")

	_, err := env.runErr("revert", "docs/readme", "0")
	if err == nil {
		t.Error("Revert(version 0) = nil, want error")
	}

	_, err = env.runErr("revert", "docs/readme", "-1")
	if err == nil {
		t.Error("Revert(version -1) = nil, want error")
	}
}
