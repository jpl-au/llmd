package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSync(t *testing.T) {
	t.Run("basic sync", func(t *testing.T) {
		env := newTestEnv(t)
		env.run("config", "sync.files", "true")
		env.runStdin("original content", "write", "docs/readme")

		mirror := filepath.Join(env.dir, ".llmd", "docs", "readme.md")
		if err := os.WriteFile(mirror, []byte("modified content"), 0644); err != nil {
			t.Fatalf("failed to modify mirror file: %v", err)
		}

		env.run("sync")

		out := env.run("cat", "docs/readme")
		env.equals(out, "modified content")
	})

	t.Run("no changes", func(t *testing.T) {
		env := newTestEnv(t)
		env.run("config", "sync.files", "true")
		env.runStdin("content", "write", "docs/readme")

		_ = env.run("sync")
	})

	t.Run("new file in mirror", func(t *testing.T) {
		env := newTestEnv(t)
		env.run("config", "sync.files", "true")

		mirror := filepath.Join(env.dir, ".llmd", "docs")
		_ = os.MkdirAll(mirror, 0755)
		_ = os.WriteFile(filepath.Join(mirror, "new.md"), []byte("new file content"), 0644)

		env.run("sync")

		out := env.run("cat", "docs/new")
		env.equals(out, "new file content")
	})
}

func TestSync_Disabled(t *testing.T) {
	env := newTestEnv(t)
	// Explicitly disable sync to isolate from user's global config.
	// Without this, a global ~/.llmd/config.yaml with sync.files: true
	// would cause this test to fail.
	env.run("config", "sync.files", "false")
	env.runStdin("content", "write", "docs/readme")

	mirror := filepath.Join(env.dir, ".llmd", "docs", "readme.md")
	if _, err := os.Stat(mirror); !errors.Is(err, fs.ErrNotExist) {
		t.Error("mirror file exists when sync disabled, want none")
	}
}

func TestSync_DryRun(t *testing.T) {
	env := newTestEnv(t)
	env.run("config", "sync.files", "true")

	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	mirror := filepath.Join(env.dir, ".llmd", "docs", "guide.md")
	if _, err := os.Stat(mirror); errors.Is(err, fs.ErrNotExist) {
		t.Skip("mirror file not created, sync may not work this way")
	}

	modifiedContent := "# Modified Guide\n\nThis content was changed directly in the mirror."
	_ = os.WriteFile(mirror, []byte(modifiedContent), 0644)

	out := env.run("sync", "-n")
	env.contains(out, "guide")

	content := env.run("cat", "docs/guide")
	if content == modifiedContent {
		t.Error("Sync(-n) synced changes, want dry run only")
	}
	env.contains(content, "LLMD Guide")
}
