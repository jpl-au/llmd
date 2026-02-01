//go:build ignore

// Build script for llmd. Cross-platform, no shell dependencies.
//
// This script builds both the llmd host binary and its plugins. It handles
// the complexity of building WASM plugins with the correct flags and injecting
// version information into the host binary.
//
// # Usage
//
//	go run tools/build/main.go              # Build everything (plugins + host with version)
//	go run tools/build/main.go plugins      # Build plugins only
//	go run tools/build/main.go host         # Build host only
//
// # Plugin Building
//
// Plugins are built as WASM reactor modules using:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o embed/core.wasm .
//
// The -buildmode=c-shared flag is critical: it creates a "reactor" module that
// initialises but doesn't exit, allowing the host to call exports. Without this
// flag, the module would run main() and exit immediately.
//
// # Host Building
//
// The host binary is built with version information injected via ldflags:
//
//	go build -ldflags "-X main.Version=... -X main.Commit=..." -o llmd ./cmd/llmd
//
// Version information is extracted from git tags and commits.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// debugMode is set by parsing --debug flag
var debugMode bool

// main is the entry point for the build script.
// It parses command-line arguments to determine what to build.
func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	// Parse args - default to building everything
	target := "all"
	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-debug" {
			debugMode = true
		} else {
			target = arg
		}
	}

	switch target {
	case "all":
		buildPlugins(root)
		buildHost(root)
	case "plugins":
		buildPlugins(root)
	case "host":
		buildHost(root)
	default:
		fmt.Fprintf(os.Stderr, "Unknown target: %s\n", target)
		fmt.Fprintf(os.Stderr, "Usage: go run tools/build/main.go [all|plugins|host] [--debug]\n")
		os.Exit(1)
	}
}

// buildPlugins builds all WASM plugins.
//
// Each plugin is built as a reactor WASM module using -buildmode=c-shared.
// This is required because plugins need to initialise (running init()) and
// then stay alive for the host to call their exports. Without c-shared mode,
// the WASM module would run main() and exit immediately.
//
// The output is placed in embed/ so it can be embedded in the host binary
// using Go's embed package.
func buildPlugins(root string) {
	plugins := []struct {
		name string
		src  string
		out  string
	}{
		{"core", "plugins/core", "embed/core.wasm"},
	}

	for _, p := range plugins {
		fmt.Printf("Building %s plugin...\n", p.name)

		outPath := filepath.Join(root, p.out)

		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
			os.Exit(1)
		}

		// Build with c-shared mode for reactor WASM modules
		cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", outPath, ".")
		cmd.Dir = filepath.Join(root, p.src)
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build %s: %v\n", p.name, err)
			os.Exit(1)
		}

		// Report size for visibility
		if info, err := os.Stat(outPath); err == nil {
			fmt.Printf("  → %s (%d KB)\n", p.out, info.Size()/1024)
		}
	}
}

// buildHost builds the llmd host binary with version information.
//
// Version information (git tag, commit hash, build time, Go version) is
// injected at compile time using ldflags. The -s -w flags strip debug
// information and symbol tables for a smaller binary (unless debug mode).
func buildHost(root string) {
	if debugMode {
		fmt.Println("Building llmd (debug)...")
	} else {
		fmt.Println("Building llmd...")
	}

	version := getGitTag()
	commit := getGitCommit()
	buildTime := time.Now().UTC().Format("2006-01-02_15:04:05_UTC")
	goVersion := runtime.Version()

	// Inject version info via ldflags
	pkg := "github.com/jpl-au/llmd/internal/version"
	debugPkg := "github.com/jpl-au/llmd/internal/debug"

	var ldflags string
	if debugMode {
		// Debug build: include debug symbols and enable debug logging
		ldflags = fmt.Sprintf(
			"-X %s.Tag=%s -X %s.Commit=%s -X %s.BuildTime=%s -X %s.GoVersion=%s -X %s.Debug=true",
			pkg, version, pkg, commit, pkg, buildTime, pkg, goVersion, debugPkg,
		)
	} else {
		// Release build: -s strips symbol table, -w strips DWARF debug info
		ldflags = fmt.Sprintf(
			"-s -w -X %s.Tag=%s -X %s.Commit=%s -X %s.BuildTime=%s -X %s.GoVersion=%s",
			pkg, version, pkg, commit, pkg, buildTime, pkg, goVersion,
		)
	}

	// Determine output binary name (add .exe on Windows)
	output := "llmd"
	if runtime.GOOS == "windows" {
		output = "llmd.exe"
	}

	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", output, ".")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build host: %v\n", err)
		os.Exit(1)
	}

	suffix := ""
	if debugMode {
		suffix = " [debug]"
	}
	fmt.Printf("Built %s (%s)%s\n", output, commit, suffix)
}

// getGitTag returns the current git version.
//
// Uses git describe to get a version string like "v0.1.0" (if on a tag)
// or "v0.1.0-5-gabc123" (if 5 commits ahead of tag v0.1.0 at commit abc123).
// Returns "dev" if git is not available or no tags exist.
func getGitTag() string {
	out, err := exec.Command("git", "describe", "--tags", "--always").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}

// getGitCommit returns the current git commit hash (short form).
//
// Returns the 7-character abbreviated commit hash for display purposes.
// Returns "unknown" if git is not available.
func getGitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
