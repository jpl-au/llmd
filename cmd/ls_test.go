package cmd

import (
	"strings"
	"testing"
)

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

		out := env.run("ls")
		env.contains(out, "docs/readme")
		env.contains(out, "docs/api")
		env.contains(out, "notes/meeting")
	})

	t.Run("path prefix filter", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")
		env.runStdin("content", "write", "docs/api")
		env.runStdin("content", "write", "notes/meeting")

		out := env.run("ls", "docs/")
		env.contains(out, "docs/readme")
		env.contains(out, "docs/api")
		if strings.Contains(out, "notes/meeting") {
			t.Error("Ls(docs/) contains notes/meeting, want excluded")
		}
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme")

		out := env.run("ls", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, "docs/readme")
	})
}

func TestLs_Formats(t *testing.T) {
	t.Run("long format", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme", "-a", "alice")

		out := env.run("ls", "-l")
		env.contains(out, "docs/readme")
		env.contains(out, "alice")
	})

	t.Run("tree format", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/api/auth")
		env.runStdin("content", "write", "docs/api/users")
		env.runStdin("content", "write", "docs/readme")

		out := env.run("ls", "-t")
		env.contains(out, "docs")
	})
}

func TestLs_Deleted(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")
	env.runStdin("content", "write", "docs/old")
	env.run("rm", "docs/old")

	tests := []struct {
		name        string
		flag        string
		wantMatch   []string
		wantNoMatch []string
	}{
		{
			name:        "default excludes deleted",
			flag:        "",
			wantMatch:   []string{"docs/readme"},
			wantNoMatch: []string{"docs/old"},
		},
		{
			name:        "-D shows only deleted",
			flag:        "-D",
			wantMatch:   []string{"docs/old"},
			wantNoMatch: []string{"docs/readme"},
		},
		{
			name:      "-A shows all",
			flag:      "-A",
			wantMatch: []string{"docs/readme", "docs/old"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out string
			if tc.flag == "" {
				out = env.run("ls")
			} else {
				out = env.run("ls", tc.flag)
			}

			for _, want := range tc.wantMatch {
				env.contains(out, want)
			}
			for _, noWant := range tc.wantNoMatch {
				if strings.Contains(out, noWant) {
					t.Errorf("Ls(%s) contains %q, want excluded", tc.flag, noWant)
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
		name        string
		tag         string
		wantMatch   []string
		wantNoMatch []string
	}{
		{
			name:        "filter by important",
			tag:         "important",
			wantMatch:   []string{"docs/guide", "docs/api"},
			wantNoMatch: []string{"notes/meeting"},
		},
		{
			name:        "filter by draft",
			tag:         "draft",
			wantMatch:   []string{"notes/meeting"},
			wantNoMatch: []string{"docs/guide", "docs/api"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := env.run("ls", "--tag", tc.tag)

			for _, want := range tc.wantMatch {
				env.contains(out, want)
			}
			for _, noWant := range tc.wantNoMatch {
				if strings.Contains(out, noWant) {
					t.Errorf("Ls(--tag %s) contains %q, want excluded", tc.tag, noWant)
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

		out := env.run("ls", "-s", "name", "-R")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) >= 3 {
			// Format is "KEY  PATH", check that paths are in reverse alphabetical order
			if !strings.HasSuffix(lines[0], "zebra") {
				t.Errorf("Ls(-s name -R) first = %q, want to end with zebra", lines[0])
			}
			if !strings.HasSuffix(lines[2], "apple") {
				t.Errorf("Ls(-s name -R) last = %q, want to end with apple", lines[2])
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
		out := env.run("ls", "-s", "time", "-R")
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
