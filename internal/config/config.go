// Package config reads and writes llmd configuration files.
//
// Config uses a simple "key=value" format. Two files are merged:
// global (~/.llmd/config) and local (.llmd/config), with local
// values taking precedence.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load merges global and local configuration. Local values override
// global ones, so a project can set its own author without affecting
// other stores.
func Load() map[string]string {
	cfg := make(map[string]string)

	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".llmd", "config")
	loadFile(globalPath, cfg)

	// Local overrides global.
	loadFile(".llmd/config", cfg)

	return cfg
}

// Save writes a key=value to a config file, preserving any existing
// values. When global is true it writes to ~/.llmd/config;
// otherwise it writes to the local .llmd/config.
func Save(key, value string, global bool) error {
	path := ".llmd/config"
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".llmd", "config")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	cfg := make(map[string]string)
	loadFile(path, cfg)
	cfg[key] = value

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range cfg {
		fmt.Fprintf(f, "%s=%s\n", k, v)
	}
	return nil
}

// Path returns the config file path that would be used for writes.
// Local .llmd/config takes precedence over global config.
func Path() string {
	if _, err := os.Stat(".llmd/config"); err == nil {
		return ".llmd/config"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".llmd", "config")
}

// loadFile reads a "key=value" config file into cfg. Missing files
// are silently ignored (config is optional).
func loadFile(path string, cfg map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			cfg[key] = value
		}
	}
}
