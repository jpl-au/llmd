package cmd

import (
	"strings"
	"testing"
)

const readme = `# Project README

Welcome to the project. This document provides an overview.

## Installation

Run the following command:
` + "```" + `bash
go install github.com/example/project@latest
` + "```" + `

## Usage

See the documentation for detailed usage instructions.

## Contributing

Pull requests are welcome. Please read CONTRIBUTING.md first.
`

func TestCat(t *testing.T) {
	t.Run("basic output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(readme, "write", "docs/readme")

		out := env.run("cat", "docs/readme")
		env.contains(out, "# Project README")
		env.contains(out, "## Installation")
		env.contains(out, "go install")
	})

	t.Run("line numbers", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(readme, "write", "docs/readme")

		out := env.run("cat", "-n", "docs/readme")
		env.contains(out, "1")
		env.contains(out, "# Project README")
		lines := strings.Split(out, "\n")
		if len(lines) < 10 {
			t.Errorf("Cat(-n) = %d lines, want >= 10", len(lines))
		}
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(readme, "write", "docs/readme")

		out := env.run("cat", "-o", "json", "docs/readme")
		env.contains(out, `"path"`)
		env.contains(out, `"content"`)
		env.contains(out, `"version"`)
		env.contains(out, "Project README")
	})

	t.Run("not found", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("cat", "docs/nonexistent")
		if err == nil {
			t.Error("Cat(nonexistent) = nil, want error")
		}
	})
}

func TestCat_Versions(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"latest", "", "# Version 2\n\nUpdated content with more details."},
		{"v1", "1", "# Version 1\n\nInitial content."},
		{"v2", "2", "# Version 2\n\nUpdated content with more details."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.runStdin("# Version 1\n\nInitial content.", "write", "docs/readme")
			env.runStdin("# Version 2\n\nUpdated content with more details.", "write", "docs/readme")

			var out string
			if tc.version == "" {
				out = env.run("cat", "docs/readme")
			} else {
				out = env.run("cat", "-v", tc.version, "docs/readme")
			}
			env.equals(out, tc.want)
		})
	}
}

func TestCat_Deleted(t *testing.T) {
	t.Run("without flag fails", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(readme, "write", "docs/readme")
		env.run("rm", "docs/readme")

		_, err := env.runErr("cat", "docs/readme")
		if err == nil {
			t.Error("Cat(deleted) = nil, want error")
		}
	})

	t.Run("with flag succeeds", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(readme, "write", "docs/readme")
		env.run("rm", "docs/readme")

		out := env.run("cat", "--deleted", "docs/readme")
		env.contains(out, "Project README")
	})
}

func TestCat_MultipleVersions(t *testing.T) {
	env := newTestEnv(t)

	env.runStdin("Draft 1", "write", "docs/spec", "-a", "alice")
	env.runStdin("Draft 2 - reviewed", "write", "docs/spec", "-a", "bob")
	env.runStdin("Final version", "write", "docs/spec", "-a", "alice")

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"v1", "1", "Draft 1"},
		{"v2", "2", "Draft 2"},
		{"v3", "3", "Final version"},
		{"latest", "", "Final version"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out string
			if tc.version == "" {
				out = env.run("cat", "docs/spec")
			} else {
				out = env.run("cat", "-v", tc.version, "docs/spec")
			}
			env.contains(out, tc.want)
		})
	}
}

func TestCat_TrailingNewline(t *testing.T) {
	// Verify no phantom lines when content ends with newline
	t.Run("line count with trailing newline", func(t *testing.T) {
		env := newTestEnv(t)
		// 3 actual lines + trailing newline
		content := "line1\nline2\nline3\n"
		env.runStdin(content, "write", "test")

		out := env.run("cat", "-n", "test")
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")

		// Should have exactly 3 lines, not 4 (no phantom)
		if len(lines) != 3 {
			t.Errorf("Cat(-n) with trailing newline: got %d lines, want 3\nOutput: %q", len(lines), out)
		}

		// Each line should have content, not be empty
		for i, line := range lines {
			if !strings.Contains(line, "line") {
				t.Errorf("Line %d appears empty or malformed: %q", i+1, line)
			}
		}
	})

	t.Run("line count without trailing newline", func(t *testing.T) {
		env := newTestEnv(t)
		// 3 lines, no trailing newline
		content := "line1\nline2\nline3"
		env.runStdin(content, "write", "test")

		out := env.run("cat", "-n", "test")
		lines := strings.Split(out, "\n")

		// Filter empty trailing element from split
		nonEmpty := 0
		for _, l := range lines {
			if l != "" {
				nonEmpty++
			}
		}

		if nonEmpty != 3 {
			t.Errorf("Cat(-n) without trailing newline: got %d non-empty lines, want 3\nOutput: %q", nonEmpty, out)
		}
	})
}

func TestCat_Lines(t *testing.T) {
	const multiLine = `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7`

	t.Run("range start:end", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(multiLine, "write", "docs/test")

		out := env.run("cat", "-l", "2:4", "docs/test")
		env.contains(out, "Line 2")
		env.contains(out, "Line 3")
		env.contains(out, "Line 4")
		if strings.Contains(out, "Line 1") {
			t.Error("Cat(-l 2:4) contains Line 1, want excluded")
		}
		if strings.Contains(out, "Line 5") {
			t.Error("Cat(-l 2:4) contains Line 5, want excluded")
		}
	})

	t.Run("range start:", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(multiLine, "write", "docs/test")

		out := env.run("cat", "-l", "5:", "docs/test")
		env.contains(out, "Line 5")
		env.contains(out, "Line 6")
		env.contains(out, "Line 7")
		if strings.Contains(out, "Line 4") {
			t.Error("Cat(-l 5:) contains Line 4, want excluded")
		}
	})

	t.Run("range :end", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(multiLine, "write", "docs/test")

		out := env.run("cat", "-l", ":3", "docs/test")
		env.contains(out, "Line 1")
		env.contains(out, "Line 2")
		env.contains(out, "Line 3")
		if strings.Contains(out, "Line 4") {
			t.Error("Cat(-l :3) contains Line 4, want excluded")
		}
	})

	t.Run("with line numbers", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(multiLine, "write", "docs/test")

		out := env.run("cat", "-n", "-l", "3:5", "docs/test")
		env.contains(out, "3")
		env.contains(out, "Line 3")
		env.contains(out, "5")
		env.contains(out, "Line 5")
	})
}

func TestCat_VersionValidation(t *testing.T) {
	t.Run("negative version rejected", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(readme, "write", "docs/readme")

		_, err := env.runErr("cat", "-v", "-1", "docs/readme")
		if err == nil {
			t.Error("Cat(-v -1) should fail")
		}
	})
}

func TestCat_MultipleFiles(t *testing.T) {
	t.Run("concatenates output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Content of file A", "write", "docs/a")
		env.runStdin("Content of file B", "write", "docs/b")
		env.runStdin("Content of file C", "write", "docs/c")

		out := env.run("cat", "docs/a", "docs/b", "docs/c")
		env.contains(out, "Content of file A")
		env.contains(out, "Content of file B")
		env.contains(out, "Content of file C")
	})

	t.Run("JSON returns array for multiple files", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("File one", "write", "docs/one")
		env.runStdin("File two", "write", "docs/two")

		out := env.run("cat", "-o", "json", "docs/one", "docs/two")
		// Should be an array
		if !strings.HasPrefix(strings.TrimSpace(out), "[") {
			t.Errorf("Cat JSON multiple files should return array, got: %s", out[:50])
		}
		env.contains(out, "File one")
		env.contains(out, "File two")
	})

	t.Run("JSON returns object for single file", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Single file", "write", "docs/single")

		out := env.run("cat", "-o", "json", "docs/single")
		// Should be an object, not array
		if !strings.HasPrefix(strings.TrimSpace(out), "{") {
			t.Errorf("Cat JSON single file should return object, got: %s", out[:50])
		}
	})

	t.Run("fails on first missing file", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("Exists", "write", "docs/exists")

		_, err := env.runErr("cat", "docs/exists", "docs/missing")
		if err == nil {
			t.Error("Cat with missing file should fail")
		}
	})
}
