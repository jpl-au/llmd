package cmd

import (
	"strings"
	"testing"
)

func TestLs_Recursive(t *testing.T) {
	t.Run("without -R only shows direct children", func(t *testing.T) {
		env := newTestEnv(t)
		// Create top-level doc and nested doc
		env.runStdin("top level content", "write", "readme")
		env.runStdin("nested content", "write", "docs/api")

		// Without -R, should only find top-level doc
		out := env.run("ls")
		env.contains(out, "readme")
		if strings.Contains(out, "docs/api") {
			t.Error("Ls without -R found nested doc, want only direct children")
		}
	})

	t.Run("with -R shows all nested documents", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("top level content", "write", "readme")
		env.runStdin("nested content", "write", "docs/api")

		// With -R, should find both
		out := env.run("ls", "-R")
		env.contains(out, "readme")
		env.contains(out, "docs/api")
	})

	t.Run("without -R with path prefix shows direct children of prefix", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("api content", "write", "docs/api")
		env.runStdin("nested meeting", "write", "docs/notes/meeting")

		// Without -R, docs/ should only find docs/api, not docs/notes/meeting
		out := env.run("ls", "docs/")
		env.contains(out, "docs/api")
		if strings.Contains(out, "docs/notes/meeting") {
			t.Error("Ls docs/ without -R found deeply nested doc")
		}
	})

	t.Run("with -R and path prefix shows all under prefix", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("api content", "write", "docs/api")
		env.runStdin("nested meeting", "write", "docs/notes/meeting")

		// With -R, should find both under docs/
		out := env.run("ls", "-R", "docs/")
		env.contains(out, "docs/api")
		env.contains(out, "docs/notes/meeting")
	})
}

func TestLs(t *testing.T) {
	t.Run("empty store", func(t *testing.T) {
		env := newTestEnv(t)

		out := env.run("ls")
		if strings.TrimSpace(out) != "" && !strings.Contains(out, "No documents") {
			t.Errorf("Ls() = %q, want empty or 'No documents'", out)
		}
	})

	t.Run("basic listing", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content 1", "write", "docs/readme")
		env.runStdin("content 2", "write", "docs/api")
		env.runStdin("content 3", "write", "notes/meeting")

		out := env.run("ls", "-R")
		env.contains(out, "docs/readme")
		env.contains(out, "docs/api")
		env.contains(out, "notes/meeting")
	})

	t.Run("path prefix filter", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api")
		env.runStdin("content", "write", "notes/meeting")

		out := env.run("ls", "-R", "docs/")
		env.contains(out, "docs/readme")
		env.contains(out, "docs/api")
		if strings.Contains(out, "notes/meeting") {
			t.Error("Ls(docs/) contains notes/meeting, want excluded")
		}
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		out := env.run("ls", "-R", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, "docs/readme")
	})
}

func TestLs_Formats(t *testing.T) {
	t.Run("long format", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme", "-a", "alice")

		out := env.run("ls", "-R", "-l")
		env.contains(out, "docs/readme")
		env.contains(out, "alice")
	})

	t.Run("tree format", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/api/auth")
		env.runStdin("content", "write", "docs/api/users")
		env.runStdin("content", "write", "docs/readme")

		out := env.run("ls", "-R", "-t")
		env.contains(out, "docs")
	})
}

func TestLs_Deleted(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")
	env.runStdin("content", "write", "docs/old")
	env.run("rm", "docs/old")

	tests := []struct {
		name    string
		flags   []string
		want    []string
		exclude []string
	}{
		{
			name:    "default excludes deleted",
			flags:   []string{"-R"},
			want:    []string{"docs/readme"},
			exclude: []string{"docs/old"},
		},
		{
			name:    "-D shows only deleted",
			flags:   []string{"-R", "-D"},
			want:    []string{"docs/old"},
			exclude: []string{"docs/readme"},
		},
		{
			name:  "-A shows all",
			flags: []string{"-R", "-A"},
			want:  []string{"docs/readme", "docs/old"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"ls"}, tc.flags...)
			out := env.run(args...)

			for _, want := range tc.want {
				env.contains(out, want)
			}
			for _, s := range tc.exclude {
				if strings.Contains(out, s) {
					t.Errorf("Ls(%v) contains %q, want excluded", tc.flags, s)
				}
			}
		})
	}
}

func TestLs_Tag(t *testing.T) {
	env := newTestEnv(t)

	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")
	env.runStdin("API documentation content", "write", "docs/api")
	env.runStdin("Meeting notes content", "write", "notes/meeting")

	env.run("tag", "add", "docs/guide", "important")
	env.run("tag", "add", "docs/api", "important")
	env.run("tag", "add", "notes/meeting", "draft")

	tests := []struct {
		name    string
		tag     string
		want    []string
		exclude []string
	}{
		{
			name:    "filter by important",
			tag:     "important",
			want:    []string{"docs/guide", "docs/api"},
			exclude: []string{"notes/meeting"},
		},
		{
			name:    "filter by draft",
			tag:     "draft",
			want:    []string{"notes/meeting"},
			exclude: []string{"docs/guide", "docs/api"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := env.run("ls", "-R", "--tag", tc.tag)

			for _, want := range tc.want {
				env.contains(out, want)
			}
			for _, s := range tc.exclude {
				if strings.Contains(out, s) {
					t.Errorf("Ls(--tag %s) contains %q, want excluded", tc.tag, s)
				}
			}
		})
	}
}

func TestLs_Sort(t *testing.T) {
	t.Run("sort by name", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "zebra")
		env.runStdin("content", "write", "apple")
		env.runStdin("content", "write", "middle")

		out := env.run("ls", "-s", "name")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) >= 3 {
			// Format is "KEY  PATH", check that paths are in alphabetical order
			if !strings.HasSuffix(lines[0], "apple") {
				t.Errorf("Ls(-s name) first = %q, want to end with apple", lines[0])
			}
			if !strings.HasSuffix(lines[2], "zebra") {
				t.Errorf("Ls(-s name) last = %q, want to end with zebra", lines[2])
			}
		}
	})

	t.Run("sort by name reverse", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "zebra")
		env.runStdin("content", "write", "apple")
		env.runStdin("content", "write", "middle")

		out := env.run("ls", "-R", "-s", "name", "-r")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) >= 3 {
			// Format is "KEY  PATH", check that paths are in reverse alphabetical order
			if !strings.HasSuffix(lines[0], "zebra") {
				t.Errorf("Ls(-s name -r) first = %q, want to end with zebra", lines[0])
			}
			if !strings.HasSuffix(lines[2], "apple") {
				t.Errorf("Ls(-s name -r) last = %q, want to end with apple", lines[2])
			}
		}
	})

	t.Run("sort by time", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "aaa")
		env.runStdin("content", "write", "bbb")
		env.runStdin("content", "write", "ccc")

		// Time sort should return documents (order may vary if same timestamp)
		// Just verify the flag works and returns all documents
		out := env.run("ls", "-s", "time")
		env.contains(out, "aaa")
		env.contains(out, "bbb")
		env.contains(out, "ccc")
	})

	t.Run("sort by time reverse", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "aaa")
		env.runStdin("content", "write", "bbb")
		env.runStdin("content", "write", "ccc")

		// Time reverse sort should return documents (order may vary if same timestamp)
		out := env.run("ls", "-R", "-s", "time", "-r")
		env.contains(out, "aaa")
		env.contains(out, "bbb")
		env.contains(out, "ccc")
	})

	t.Run("invalid sort field rejected", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "test")

		_, err := env.runErr("ls", "-s", "invalid")
		if err == nil {
			t.Error("Ls(-s invalid) should fail")
		}
	})
}
