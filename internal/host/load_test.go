package host

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirNotExist(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	// Non-existent directory should be skipped silently
	err = h.load(ctx, PluginDir, "/nonexistent/path/to/plugins")
	if err != nil {
		t.Errorf("loadDir non-existent: got error %v, want nil", err)
	}
}

func TestLoadDirEmpty(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	// Create empty temp directory
	dir := t.TempDir()

	err = h.load(ctx, PluginDir, dir)
	if err != nil {
		t.Errorf("loadDir empty: got error %v, want nil", err)
	}
}

func TestLoadDirSkipsNonWasm(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	// Create temp directory with non-wasm files
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "plugin.so"), []byte("not wasm"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	err = h.load(ctx, PluginDir, dir)
	if err != nil {
		t.Errorf("loadDir with non-wasm files: got error %v, want nil", err)
	}

	// Should have loaded no plugins
	if len(h.plugins) != 0 {
		t.Errorf("plugins count: got %d, want 0", len(h.plugins))
	}
}

func TestLoadDirInvalidWasm(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	// Create temp directory with invalid wasm file
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.wasm"), []byte("not valid wasm"), 0644)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err = h.load(ctx, PluginDir, dir)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()

	// Should not return error (continues after warning)
	if err != nil {
		t.Errorf("loadDir invalid wasm: got error %v, want nil", err)
	}

	// Should have printed warning
	if !strings.Contains(stderr, "warning") {
		t.Errorf("stderr: expected warning, got %q", stderr)
	}

	// Should have loaded no plugins
	if len(h.plugins) != 0 {
		t.Errorf("plugins count: got %d, want 0", len(h.plugins))
	}
}

func TestLoadFileMissing(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	err = h.load(ctx, PluginFile, "/nonexistent/plugin.wasm")
	if err == nil {
		t.Error("loadFile missing: got nil, want error")
	}
}

func TestLoadFileInvalid(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	// Create invalid wasm file
	f := filepath.Join(t.TempDir(), "bad.wasm")
	os.WriteFile(f, []byte("not valid wasm"), 0644)

	err = h.load(ctx, PluginFile, f)
	if err == nil {
		t.Error("loadFile invalid: got nil, want error")
	}
}

func TestLoadEmbed(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	err = h.load(ctx, PluginEmbed, "core")
	if err != nil {
		t.Fatalf("loadEmbed: %v", err)
	}

	// Should have loaded core plugin
	if len(h.plugins) != 1 {
		t.Errorf("plugins count: got %d, want 1", len(h.plugins))
	}

	if h.plugins[0].Name != "core" {
		t.Errorf("plugin name: got %q, want %q", h.plugins[0].Name, "core")
	}

	if h.plugins[0].Source != "embedded" {
		t.Errorf("plugin source: got %q, want %q", h.plugins[0].Source, "embedded")
	}

	// Should have registered commands
	if len(h.commands) == 0 {
		t.Error("commands: got 0, want >0")
	}

	// Check for expected command
	if _, ok := h.commands["cat"]; !ok {
		t.Error("commands: missing 'cat'")
	}
}

func TestLoadPlugins(t *testing.T) {
	ctx := context.Background()
	h, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close(ctx)

	err = h.LoadPlugins(ctx)
	if err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	// Should have loaded core plugin
	if len(h.plugins) == 0 {
		t.Error("plugins: got 0, want >0")
	}

	// Should have registered commands
	if len(h.commands) == 0 {
		t.Error("commands: got 0, want >0")
	}
}
