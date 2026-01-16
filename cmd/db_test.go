package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB(t *testing.T) {
	t.Run("list databases", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		// Init two databases
		cmd := exec.Command(binary, "init")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		cmd = exec.Command(binary, "init", "--db", "docs")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "init --db docs failed: %s", out)

		// List databases
		cmd = exec.Command(binary, "db")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db failed: %s", out)

		assert.Contains(t, string(out), "llmd.db")
		assert.Contains(t, string(out), "llmd-docs.db")
		assert.Contains(t, string(out), "shared")
	})

	t.Run("mark as local", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init", "--db", "notes")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		// Mark as local
		cmd = exec.Command(binary, "db", "notes", "--local")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db notes --local failed: %s", out)

		assert.Contains(t, string(out), "marked as local")

		// Verify gitignore
		gitignore, err := os.ReadFile(filepath.Join(dir, ".llmd", ".gitignore"))
		require.NoError(t, err)
		assert.Contains(t, string(gitignore), "llmd-notes.db")
	})

	t.Run("mark as shared", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		// Init as local
		cmd := exec.Command(binary, "init", "--db", "notes", "--local")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		// Verify in gitignore
		gitignore, err := os.ReadFile(filepath.Join(dir, ".llmd", ".gitignore"))
		require.NoError(t, err)
		assert.Contains(t, string(gitignore), "llmd-notes.db")

		// Mark as shared
		cmd = exec.Command(binary, "db", "notes", "--share")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db notes --share failed: %s", out)

		assert.Contains(t, string(out), "marked as shared")

		// Verify removed from gitignore
		gitignore, err = os.ReadFile(filepath.Join(dir, ".llmd", ".gitignore"))
		require.NoError(t, err)
		assert.NotContains(t, string(gitignore), "llmd-notes.db")
	})

	t.Run("show status", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init", "--db", "notes", "--local")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		// Check status
		cmd = exec.Command(binary, "db", "notes")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db notes failed: %s", out)

		assert.Contains(t, string(out), "local")
	})

	t.Run("cannot use local and share together", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init", "--db", "notes")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		cmd = exec.Command(binary, "db", "notes", "--local", "--share")
		cmd.Dir = dir
		_, err = cmd.CombinedOutput()
		assert.Error(t, err)
	})

	t.Run("--dir targets external directory", func(t *testing.T) {
		// Test that db --dir works with an external project directory
		currentDir := t.TempDir()
		externalDir := t.TempDir()
		binary := buildBinary(t)

		// Init database in external directory
		cmd := exec.Command(binary, "init", "--dir", externalDir)
		cmd.Dir = currentDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init --dir failed: %s", out)

		// List databases using --dir from a different directory
		cmd = exec.Command(binary, "db", "--dir", externalDir)
		cmd.Dir = currentDir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db --dir failed: %s", out)
		assert.Contains(t, string(out), "llmd.db")

		// Mark default database as local using --dir (no name = default)
		cmd = exec.Command(binary, "db", "--dir", externalDir, "--local")
		cmd.Dir = currentDir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db --dir --local failed: %s", out)

		// Verify gitignore was updated in external directory
		gitignore, err := os.ReadFile(filepath.Join(externalDir, ".llmd", ".gitignore"))
		require.NoError(t, err)
		assert.Contains(t, string(gitignore), "llmd.db")
	})

	t.Run("--local without name defaults to default database", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		// Mark default database as local without specifying a name
		cmd = exec.Command(binary, "db", "--local")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "db --local failed: %s", out)
		assert.Contains(t, string(out), "llmd.db")
		assert.Contains(t, string(out), "marked as local")

		// Verify gitignore
		gitignore, err := os.ReadFile(filepath.Join(dir, ".llmd", ".gitignore"))
		require.NoError(t, err)
		assert.Contains(t, string(gitignore), "llmd.db")
	})
}
