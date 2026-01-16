package cmd

import (
	"strings"
	"testing"
)

func TestRm(t *testing.T) {
	t.Run("basic delete", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		env.run("rm", "docs/readme")

		out := env.run("ls")
		if strings.Contains(out, "docs/readme") {
			t.Error("Rm() doc still visible, want deleted")
		}

		out = env.run("ls", "-D")
		env.contains(out, "docs/readme")
	})

	t.Run("not found", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("rm", "docs/nonexistent")
		if err == nil {
			t.Error("Rm(nonexistent) = nil, want error")
		}
	})

	t.Run("already deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.run("rm", "docs/readme")

		_, err := env.runErr("rm", "docs/readme")
		if err == nil {
			t.Error("Rm(already deleted) = nil, want error")
		}
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/rm")

		out := env.run("rm", "docs/rm", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/rm"`)
		env.contains(out, `"deleted"`)
	})
}

func TestRm_Recursive(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/api/auth")
	env.runStdin("content", "write", "docs/api/users")
	env.runStdin("content", "write", "docs/readme")

	env.run("rm", "-r", "docs/api/")

	out := env.run("ls")
	if strings.Contains(out, "docs/api") {
		t.Error("Rm(-r) api docs still visible, want deleted")
	}
	env.contains(out, "docs/readme")
}

func TestRm_PreservesHistory(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("version 1", "write", "docs/readme")
	env.runStdin("version 2", "write", "docs/readme")

	env.run("rm", "docs/readme")

	out := env.run("history", "docs/readme", "--deleted")
	env.contains(out, "v1")
	env.contains(out, "v2")
}
