package cmd

import (
	"strings"
	"testing"
)

func TestMv(t *testing.T) {
	t.Run("basic move", func(t *testing.T) {
		env := newTestEnv(t)
		content := "original content"
		env.runStdin(content, "write", "docs/old")

		env.run("mv", "docs/old", "docs/new")

		_, err := env.runErr("cat", "docs/old")
		if err == nil {
			t.Error("Mv() old path still exists, want removed")
		}

		out := env.run("cat", "docs/new")
		env.equals(out, content)
	})

	t.Run("rename in same directory", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/README")

		env.run("mv", "docs/README", "docs/readme")

		out := env.run("ls")
		env.contains(out, "docs/readme")
		if strings.Contains(out, "docs/README") {
			t.Error("Mv() old name still visible, want removed")
		}
	})

	t.Run("change directory", func(t *testing.T) {
		env := newTestEnv(t)
		content := "api docs"
		env.runStdin(content, "write", "docs/api/readme")

		env.run("mv", "docs/api/readme", "archive/old-api")

		out := env.run("ls")
		if strings.Contains(out, "docs/api/readme") {
			t.Error("Mv() old path still visible, want removed")
		}
		env.contains(out, "archive/old-api")

		out = env.run("cat", "archive/old-api")
		env.equals(out, content)
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		out := env.run("mv", "docs/guide", "archive/old-guide", "-o", "json")
		env.contains(out, `"from"`)
		env.contains(out, `"to"`)
		env.contains(out, `"docs/guide"`)
		env.contains(out, `"archive/old-guide"`)

		_, err := env.runErr("cat", "docs/guide")
		if err == nil {
			t.Error("Mv() old path still exists, want removed")
		}

		content := env.run("cat", "archive/old-guide")
		env.contains(content, "llmd Guide")
	})
}

func TestMv_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testEnv) // optional setup before running mv
		from  string
		to    string
	}{
		{
			name: "not found",
			from: "docs/nonexistent",
			to:   "docs/new",
		},
		{
			name: "same path",
			setup: func(e *testEnv) {
				e.runStdin("content", "write", "docs/readme")
			},
			from: "docs/readme",
			to:   "docs/readme",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			if tc.setup != nil {
				tc.setup(env)
			}

			_, err := env.runErr("mv", tc.from, tc.to)
			if err == nil {
				t.Errorf("Mv(%s, %s) = nil, want error", tc.from, tc.to)
			}
		})
	}
}

func TestMv_ToExistingPath(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content 1", "write", "docs/file1")
	env.runStdin("content 2", "write", "docs/file2")

	_, err := env.runErr("mv", "docs/file1", "docs/file2")
	if err == nil {
		t.Error("Mv(to existing) = nil, want error")
	}
}

func TestMv_PreservesHistory(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/readme")
	env.runStdin("v2", "write", "docs/readme")

	env.run("mv", "docs/readme", "docs/new-readme")

	out := env.run("history", "docs/new-readme")
	env.contains(out, "v1")
	env.contains(out, "v2")
}
