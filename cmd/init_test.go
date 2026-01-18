package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	t.Run("basic init", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		assert.DirExists(t, filepath.Join(dir, ".llmd"))
		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd.db"))
		// Note: init does NOT create config.yaml - config is managed separately
		// via "llmd config". This follows the git model where init just creates
		// the repository structure.
		assert.NoFileExists(t, filepath.Join(dir, ".llmd", "config.yaml"))
	})
}

func TestInit_AlreadyInitialised(t *testing.T) {
	dir := t.TempDir()
	binary := buildBinary(t)

	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "first init failed: %s", out)

	cmd = exec.Command(binary, "init")
	cmd.Dir = dir
	_, err = cmd.CombinedOutput()
	assert.Error(t, err)
}

func TestInit_Force(t *testing.T) {
	dir := t.TempDir()
	binary := buildBinary(t)

	// First init
	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "first init failed: %s", out)

	assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd.db"))

	// Force reinit should succeed and recreate the database
	cmd = exec.Command(binary, "init", "--force")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "init --force failed: %s", out)

	assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd.db"))
}

func TestInit_DirAndLocalIncompatible(t *testing.T) {
	// --dir and --local are incompatible because:
	// - --local modifies the current project's .gitignore
	// - --dir creates the database in an external directory
	// Adding an external database to this project's gitignore makes no sense.
	dir := t.TempDir()
	targetDir := t.TempDir()
	binary := buildBinary(t)

	cmd := exec.Command(binary, "init", "--dir", targetDir, "--local")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	assert.Error(t, err, "init --dir --local should fail")
	assert.Contains(t, string(out), "cannot use --local with --dir")
}

func TestInit_Dir(t *testing.T) {
	// --dir creates the database in an external directory
	dir := t.TempDir()
	targetDir := t.TempDir()
	binary := buildBinary(t)

	cmd := exec.Command(binary, "init", "--dir", targetDir)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "init --dir failed: %s", out)

	// Database should be in target directory, not current directory
	assert.FileExists(t, filepath.Join(targetDir, ".llmd", "llmd.db"))
	assert.NoFileExists(t, filepath.Join(dir, ".llmd", "llmd.db"))
}

func TestInit_DB(t *testing.T) {
	t.Run("creates named database", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init", "--db", "docs")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init --db docs failed: %s", out)

		assert.DirExists(t, filepath.Join(dir, ".llmd"))
		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd-docs.db"))
		assert.Contains(t, string(out), "llmd-docs.db")
	})

	t.Run("multiple databases coexist", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		// Init default database
		cmd := exec.Command(binary, "init")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		// Init named database
		cmd = exec.Command(binary, "init", "--db", "notes")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "init --db notes failed: %s", out)

		// Both should exist
		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd.db"))
		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd-notes.db"))
	})

	t.Run("LLMD_DB env var", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LLMD_DB=env-test")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init with LLMD_DB failed: %s", out)

		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd-env-test.db"))
		assert.Contains(t, string(out), "llmd-env-test.db")
	})

	t.Run("flag overrides env var", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init", "--db", "flag-value")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LLMD_DB=env-value")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		// Flag should win over env var
		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd-flag-value.db"))
		assert.NoFileExists(t, filepath.Join(dir, ".llmd", "llmd-env-value.db"))
	})

	t.Run("commands use correct database", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		// Init two databases
		cmd := exec.Command(binary, "init")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init failed: %s", out)

		cmd = exec.Command(binary, "init", "--db", "other")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "init --db other failed: %s", out)

		// Configure test author locally
		cmd = exec.Command(binary, "config", "author.name", "test", "--local")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "config author failed: %s", out)

		// Write to default database
		cmd = exec.Command(binary, "write", "default-doc")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader("content for default")
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "write to default failed: %s", out)

		// Write to other database
		cmd = exec.Command(binary, "write", "--db", "other", "other-doc")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader("content for other")
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "write to other failed: %s", out)

		// Verify default database has only default-doc
		cmd = exec.Command(binary, "ls")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "ls failed: %s", out)
		assert.Contains(t, string(out), "default-doc")
		assert.NotContains(t, string(out), "other-doc")

		// Verify other database has only other-doc
		cmd = exec.Command(binary, "ls", "--db", "other")
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "ls --db other failed: %s", out)
		assert.Contains(t, string(out), "other-doc")
		assert.NotContains(t, string(out), "default-doc")
	})

	t.Run("local flag adds to gitignore", func(t *testing.T) {
		dir := t.TempDir()
		binary := buildBinary(t)

		cmd := exec.Command(binary, "init", "--db", "notes", "--local")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "init --db notes --local failed: %s", out)

		assert.FileExists(t, filepath.Join(dir, ".llmd", "llmd-notes.db"))

		gitignore, err := os.ReadFile(filepath.Join(dir, ".llmd", ".gitignore"))
		require.NoError(t, err)
		assert.Contains(t, string(gitignore), "llmd-notes.db")
	})
}
