package cmd

import (
	"strings"
	"testing"
)

func TestRestore(t *testing.T) {
	t.Run("basic restore", func(t *testing.T) {
		env := newTestEnv(t)
		content := "important content"
		env.runStdin(content, "write", "docs/readme")
		env.run("rm", "docs/readme")

		out := env.run("ls", "-R")
		if strings.Contains(out, "docs/readme") {
			t.Error("Rm() doc still visible, want deleted")
		}

		env.run("restore", "docs/readme")

		out = env.run("ls", "-R")
		env.contains(out, "docs/readme")

		out = env.run("cat", "docs/readme")
		env.equals(out, content)
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/restore")
		env.run("rm", "docs/restore")

		out := env.run("restore", "docs/restore", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/restore"`)
	})
}

func TestRestore_Errors(t *testing.T) {
	t.Run("not deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		_, err := env.runErr("restore", "docs/readme")
		if err == nil {
			t.Error("Restore(not deleted) = nil, want error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("restore", "docs/nonexistent")
		if err == nil {
			t.Error("Restore(nonexistent) = nil, want error")
		}
	})
}

func TestRestore_PreservesVersions(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/readme")
	env.runStdin("v2", "write", "docs/readme")
	env.runStdin("v3", "write", "docs/readme")
	env.run("rm", "docs/readme")
	env.run("restore", "docs/readme")

	tests := []struct {
		version string
		want    string
	}{
		{"1", "v1"},
		{"2", "v2"},
		{"3", "v3"},
	}

	for _, tc := range tests {
		t.Run("v"+tc.version, func(t *testing.T) {
			out := env.run("cat", "-v", tc.version, "docs/readme")
			env.equals(out, tc.want)
		})
	}
}

func TestRestore_DeleteAndRestoreMultiple(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")

	for i := range 3 {
		env.run("rm", "docs/readme")
		out := env.run("ls", "-R")
		if strings.Contains(out, "docs/readme") {
			t.Errorf("iteration %d: doc still visible after rm", i)
		}

		env.run("restore", "docs/readme")
		out = env.run("ls", "-R")
		env.contains(out, "docs/readme")
	}
}

func TestRestore_KeyFlag(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")
	env.run("rm", "docs/readme")

	// Get the key from history
	out := env.run("history", "docs/readme", "--deleted", "-o", "json")
	keyStart := strings.Index(out, `"key":"`) + 7
	key := out[keyStart : keyStart+8]

	// Restore using --key flag
	env.run("restore", "--key", key)

	out = env.run("ls", "-R")
	env.contains(out, "docs/readme")
}
