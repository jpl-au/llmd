package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExport(t *testing.T) {
	t.Run("single file to new path", func(t *testing.T) {
		env := newTestEnv(t)
		content := "# Exported\n\nThis will be exported."
		env.runStdin(content, "write", "docs/readme")

		dst := filepath.Join(env.dir, "output")
		env.run("export", "docs/readme", dst)

		data, err := os.ReadFile(dst + ".md")
		require.NoError(t, err, "exported file not found")
		assert.Equal(t, content, string(data))
	})

	t.Run("single file to existing dir", func(t *testing.T) {
		env := newTestEnv(t)
		content := "# Exported\n\nThis will be exported."
		env.runStdin(content, "write", "docs/readme")

		dst := filepath.Join(env.dir, "output")
		_ = os.MkdirAll(dst, 0755)
		env.run("export", "docs/readme", dst)

		data, err := os.ReadFile(filepath.Join(dst, "readme.md"))
		require.NoError(t, err, "exported file not found")
		assert.Equal(t, content, string(data))
	})

	t.Run("single file to file path", func(t *testing.T) {
		env := newTestEnv(t)
		content := "exported content"
		env.runStdin(content, "write", "docs/readme")

		dst := filepath.Join(env.dir, "output.md")
		env.run("export", "docs/readme", dst)

		data, err := os.ReadFile(dst)
		require.NoError(t, err, "exported file not found")
		assert.Equal(t, content, string(data))
	})
}

func TestExport_Directory(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("readme content", "write", "docs/readme")
	env.runStdin("api content", "write", "docs/api")
	env.runStdin("notes content", "write", "notes/meeting")

	dst := filepath.Join(env.dir, "output")
	env.run("export", "docs/", dst)

	assert.FileExists(t, filepath.Join(dst, "readme.md"))
	assert.FileExists(t, filepath.Join(dst, "api.md"))
	assert.NoFileExists(t, filepath.Join(dst, "meeting.md"))
}

func TestExport_All(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("readme", "write", "docs/readme")
	env.runStdin("meeting", "write", "notes/meeting")

	dst := filepath.Join(env.dir, "backup")
	env.run("export", "/", dst)

	assert.FileExists(t, filepath.Join(dst, "docs", "readme.md"))
	assert.FileExists(t, filepath.Join(dst, "notes", "meeting.md"))
}

func TestExport_SpecificVersion(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("version 1", "write", "docs/readme")
	env.runStdin("version 2", "write", "docs/readme")
	env.runStdin("version 3", "write", "docs/readme")

	dst := filepath.Join(env.dir, "old.md")
	env.run("export", "docs/readme", dst, "-v", "1")

	data, err := os.ReadFile(dst)
	require.NoError(t, err, "exported file not found")
	assert.Equal(t, "version 1", string(data))
}

func TestExport_Overwrite(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("new content", "write", "docs/readme")

	dst := filepath.Join(env.dir, "existing.md")
	_ = os.WriteFile(dst, []byte("old content"), 0644)

	t.Run("without force fails", func(t *testing.T) {
		_, err := env.runErr("export", "docs/readme", dst)
		assert.Error(t, err)
	})

	t.Run("with force succeeds", func(t *testing.T) {
		env.run("export", "docs/readme", dst, "--force")

		data, _ := os.ReadFile(dst)
		assert.Equal(t, "new content", string(data))
	})
}

func TestExport_NotFound(t *testing.T) {
	env := newTestEnv(t)

	dst := filepath.Join(env.dir, "output")
	_, err := env.runErr("export", "docs/nonexistent", dst)
	assert.Error(t, err)
}

func TestExport_NegativeVersion(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")

	dst := filepath.Join(env.dir, "output.md")
	_, err := env.runErr("export", "docs/readme", dst, "-v", "-1")
	assert.Error(t, err)
}

func TestExport_KeyFlag(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("version 1", "write", "docs/readme")
	env.runStdin("version 2", "write", "docs/readme")

	// Get the key for version 1
	out := env.run("cat", "docs/readme", "-v", "1", "-o", "json")
	keyStart := strings.Index(out, `"key":"`) + 7
	key := out[keyStart : keyStart+8]

	// Export using --key flag
	dst := filepath.Join(env.dir, "exported.md")
	env.run("export", "--key", key, dst)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "version 1", string(data))
}

func TestExport_ContentIntegrity(t *testing.T) {
	// Verifies that exported files contain exactly the expected content.
	// This tests the practical atomicity guarantee - files are either
	// fully written with correct content or not created at all.
	env := newTestEnv(t)

	// Create documents with varied content including special characters
	docs := map[string]string{
		"docs/unicode":    "Unicode: 日本語 émojis 🎉 and symbols © ® ™",
		"docs/multiline":  "Line 1\nLine 2\nLine 3\n\nLine 5 after blank",
		"docs/whitespace": "  leading spaces\tand\ttabs\ntrailing spaces  \n",
		"docs/large":      string(make([]byte, 100000)), // 100KB of zeros
	}

	// Fill large doc with pattern to detect truncation
	largeContent := make([]byte, 100000)
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}
	docs["docs/large"] = string(largeContent)

	for path, content := range docs {
		env.runStdin(content, "write", path)
	}

	dst := filepath.Join(env.dir, "export-test")
	env.run("export", "docs/", dst)

	// Verify each exported file matches exactly
	for path, expectedContent := range docs {
		name := filepath.Base(path) + ".md"
		exportedPath := filepath.Join(dst, name)

		data, err := os.ReadFile(exportedPath)
		require.NoError(t, err, "reading exported file %s", name)
		assert.Equal(t, expectedContent, string(data),
			"content mismatch for %s", name)
	}
}
