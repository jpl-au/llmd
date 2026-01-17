package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff(t *testing.T) {
	t.Run("latest vs previous", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("line one\nline two", "write", "docs/readme")
		env.runStdin("line one\nline two\nline three", "write", "docs/readme")

		out := env.run("diff", "docs/readme")
		env.contains(out, "+")
		env.contains(out, "line three")
	})

	t.Run("specific versions", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("version one", "write", "docs/readme")
		env.runStdin("version two", "write", "docs/readme")
		env.runStdin("version three", "write", "docs/readme")

		out := env.run("diff", "docs/readme", "-v", "1:3")
		env.contains(out, "v1")
		env.contains(out, "v3")
	})

	t.Run("two documents", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content A", "write", "docs/file-a")
		env.runStdin("content B", "write", "docs/file-b")

		out := env.run("diff", "docs/file-a", "docs/file-b")
		env.contains(out, "file-a")
		env.contains(out, "file-b")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("line one", "write", "docs/readme")
		env.runStdin("line two", "write", "docs/readme")

		out := env.run("diff", "docs/readme", "-o", "json")
		env.contains(out, `"diff"`)
	})
}

func TestDiff_FilesystemFile(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("stored content", "write", "docs/readme")

	local := filepath.Join(env.dir, "local.md")
	require.NoError(t, os.WriteFile(local, []byte("local content"), 0644))

	out := env.run("diff", "-f", local, "docs/readme")
	env.contains(out, "local")
	env.contains(out, "readme")
}

func TestDiff_NoChanges(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("same content", "write", "docs/readme")
	env.runStdin("same content", "write", "docs/readme")

	_ = env.run("diff", "docs/readme")
}

func TestDiff_Errors(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("diff", "docs/nonexistent")
		assert.Error(t, err)
	})

	t.Run("single version", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("only one version", "write", "docs/readme")

		_, _ = env.runErr("diff", "docs/readme")
	})

	t.Run("version range v1 greater than v2", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("v1", "write", "docs/readme")
		env.runStdin("v2", "write", "docs/readme")
		env.runStdin("v3", "write", "docs/readme")

		_, err := env.runErr("diff", "docs/readme", "-v", "3:1")
		assert.Error(t, err)
	})
}

func TestDiff_Deleted(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/readme")
	env.runStdin("v2", "write", "docs/readme")
	env.run("rm", "docs/readme")

	t.Run("without flag fails", func(t *testing.T) {
		_, err := env.runErr("diff", "docs/readme")
		assert.Error(t, err)
	})

	t.Run("with flag succeeds", func(t *testing.T) {
		out := env.run("diff", "docs/readme", "-D")
		env.contains(out, "v1")
	})
}
