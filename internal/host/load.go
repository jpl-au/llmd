package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/embed"
	"github.com/jpl-au/llmd/internal/debug"
	"github.com/jpl-au/llmd/proto/plugin"
)

// PluginType identifies the source type for plugin loading.
type PluginType int

const (
	// PluginEmbed loads from embedded bytes (embed.CorePlugin).
	PluginEmbed PluginType = iota
	// PluginFile loads a single .wasm file from a path.
	PluginFile
	// PluginDir loads all .wasm files from a directory.
	PluginDir
)

// load loads a plugin from the specified source.
//
// For PluginEmbed, path is used as the plugin name (e.g., "core").
// For PluginFile, path is the .wasm file path.
// For PluginDir, path is the directory containing .wasm files.
func (h *Host) load(ctx context.Context, t PluginType, path string) error {
	switch t {
	case PluginEmbed:
		return h.loadBytes(ctx, path, embed.CorePlugin, "embedded")
	case PluginFile:
		return h.loadFile(ctx, path)
	case PluginDir:
		return h.loadDir(ctx, path)
	default:
		return fmt.Errorf("unknown plugin type: %d", t)
	}
}

// loadBytes loads a plugin from WASM bytes.
func (h *Host) loadBytes(ctx context.Context, name string, wasmBytes []byte, source string) error {
	debug.Log("loadBytes", "name", name, "source", source, "size", len(wasmBytes))

	// Write to temp file for loading (the loader expects a file path)
	tmpFile, err := os.CreateTemp("", "llmd-plugin-*.wasm")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(wasmBytes); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()
	debug.Log("loadBytes", "step", "temp file written", "path", tmpFile.Name())

	// Load the plugin
	debug.Log("loadBytes", "step", "loading plugin")
	instance, err := h.runtime.Loader().Load(ctx, tmpFile.Name())
	if err != nil {
		debug.Log("loadBytes error", "step", "load", "error", err.Error())
		return fmt.Errorf("loading plugin: %w", err)
	}
	debug.Log("loadBytes", "step", "plugin loaded")

	// Initialize the plugin
	debug.Log("loadBytes", "step", "initializing plugin")
	manifest, err := instance.Init(ctx, &plugin.InitRequest{})
	if err != nil {
		debug.Log("loadBytes error", "step", "init", "error", err.Error())
		instance.Close(ctx)
		return fmt.Errorf("initializing plugin: %w", err)
	}
	debug.Log("loadBytes", "step", "plugin initialized", "manifestName", manifest.Name, "commands", len(manifest.Commands))

	// Create loaded plugin
	loaded := &LoadedPlugin{
		Name:     manifest.Name,
		Version:  manifest.Version,
		Source:   source,
		instance: instance,
	}
	h.plugins = append(h.plugins, loaded)

	// Register commands
	for _, cmd := range manifest.Commands {
		h.commands[cmd.Name] = &RegisteredCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
			Usage:       cmd.Usage,
			Plugin:      loaded,
			MCPEnabled:  cmd.McpEnabled,
			MCPName:     cmd.McpName,
			Flags:       cmd.Flags,
		}
	}

	debug.Log("loadBytes complete", "name", name)
	return nil
}

// loadFile loads a single .wasm file.
func (h *Host) loadFile(ctx context.Context, path string) error {
	debug.Log("loadFile", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading plugin file: %w", err)
	}

	// Derive name from filename without extension
	name := strings.TrimSuffix(filepath.Base(path), ".wasm")

	return h.loadBytes(ctx, name, data, "file")
}

// loadDir loads all .wasm files from a directory.
func (h *Host) loadDir(ctx context.Context, dir string) error {
	debug.Log("loadDir", "dir", dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			debug.Log("loadDir", "dir", dir, "status", "not found, skipping")
			return nil
		}
		return fmt.Errorf("reading plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := h.loadFile(ctx, path); err != nil {
			// Warn but continue loading other plugins
			fmt.Fprintf(os.Stderr, "warning: failed to load plugin %s: %v\n", path, err)
		}
	}

	return nil
}
