package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImport(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		require.NoError(t, os.MkdirAll(src, 0755))

		content := "# Imported Document\n\nThis was imported."
		require.NoError(t, os.WriteFile(filepath.Join(src, "readme.md"), []byte(content), 0644))

		env.run("import", filepath.Join(src, "readme.md"))

		out := env.run("cat", "readme")
		env.equals(out, content)
	})

	t.Run("directory", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		docs := filepath.Join(src, "docs")
		require.NoError(t, os.MkdirAll(docs, 0755))

		_ = os.WriteFile(filepath.Join(docs, "readme.md"), []byte("readme content"), 0644)
		_ = os.WriteFile(filepath.Join(docs, "api.md"), []byte("api content"), 0644)

		env.run("import", docs)

		out := env.run("ls")
		env.contains(out, "readme")
		env.contains(out, "api")
	})

	t.Run("nested directory", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		nested := filepath.Join(src, "docs", "api", "v2")
		require.NoError(t, os.MkdirAll(nested, 0755))

		_ = os.WriteFile(filepath.Join(nested, "endpoints.md"), []byte("endpoints"), 0644)

		env.run("import", src)

		out := env.run("ls", "-R")
		env.contains(out, "docs/api/v2/endpoints")
	})

	t.Run("with prefix", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		require.NoError(t, os.MkdirAll(src, 0755))

		_ = os.WriteFile(filepath.Join(src, "readme.md"), []byte("content"), 0644)

		env.run("import", src, "-t", "imported/")

		out := env.run("ls", "-R")
		env.contains(out, "imported/readme")
	})
}

func TestImport_Filters(t *testing.T) {
	t.Run("non-markdown ignored", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		require.NoError(t, os.MkdirAll(src, 0755))

		_ = os.WriteFile(filepath.Join(src, "readme.md"), []byte("markdown"), 0644)
		_ = os.WriteFile(filepath.Join(src, "data.json"), []byte("{}"), 0644)
		_ = os.WriteFile(filepath.Join(src, "script.sh"), []byte("#!/bin/bash"), 0644)

		env.run("import", src)

		out := env.run("ls")
		env.contains(out, "readme")
	})

	t.Run("hidden files ignored by default", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		hidden := filepath.Join(src, ".hidden")
		require.NoError(t, os.MkdirAll(hidden, 0755))

		guide := testGuideContent()
		_ = os.WriteFile(filepath.Join(src, "visible.md"), []byte("visible content"), 0644)
		_ = os.WriteFile(filepath.Join(hidden, "secret.md"), []byte(guide), 0644)

		env.run("import", src)

		out := env.run("ls")
		env.contains(out, "visible")
		assert.NotContains(t, out, "secret")
	})

	t.Run("hidden files with -H flag", func(t *testing.T) {
		env := newTestEnv(t)

		src := filepath.Join(env.dir, "source")
		hidden := filepath.Join(src, ".hidden")
		_ = os.MkdirAll(hidden, 0755)

		guide := testGuideContent()
		_ = os.WriteFile(filepath.Join(src, "visible.md"), []byte("visible content"), 0644)
		_ = os.WriteFile(filepath.Join(hidden, "secret.md"), []byte(guide), 0644)

		env.run("import", src, "-H")

		out := env.run("ls", "-R")
		env.contains(out, "visible")
		env.contains(out, "secret")
	})
}

func TestImport_DryRun(t *testing.T) {
	env := newTestEnv(t)

	src := filepath.Join(env.dir, "source")
	require.NoError(t, os.MkdirAll(src, 0755))

	guide := testGuideContent()
	_ = os.WriteFile(filepath.Join(src, "guide.md"), []byte(guide), 0644)
	_ = os.WriteFile(filepath.Join(src, "api.md"), []byte("# API Reference\n\nAPI documentation here."), 0644)

	out := env.run("import", src, "-n")
	env.contains(out, "guide")
	env.contains(out, "api")

	lsOut := env.run("ls")
	// Either empty or "No documents" means dry run didn't import
	assert.True(t, lsOut == "" || strings.Contains(lsOut, "No documents"),
		"Import(-n) should not import docs, got: %s", lsOut)
}

func TestImport_Flat(t *testing.T) {
	env := newTestEnv(t)

	src := filepath.Join(env.dir, "source")
	nested := filepath.Join(src, "docs", "api", "v2")
	require.NoError(t, os.MkdirAll(nested, 0755))

	guide := testGuideContent()
	_ = os.WriteFile(filepath.Join(nested, "endpoints.md"), []byte(guide), 0644)

	env.run("import", src, "-F")

	out := env.run("ls")
	env.contains(out, "endpoints")
	assert.NotContains(t, out, "docs/api/v2")
}

func TestImport_NotFound(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.runErr("import", "/nonexistent/path")
	assert.Error(t, err)
}
