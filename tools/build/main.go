//go:build ignore

// Build script for llmd.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Building llmd...")

	output := "llmd"
	if runtime.GOOS == "windows" {
		output = "llmd.exe"
	}

	cmd := exec.Command("go", "build", "-o", output, ".")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build: %v\n", err)
		os.Exit(1)
	}

	commit := getGitCommit()
	fmt.Printf("Built %s (%s)\n", output, commit)
}

func getGitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
