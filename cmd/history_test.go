package cmd

import "testing"

func TestHistory(t *testing.T) {
	t.Run("basic history", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("version 1", "write", "docs/readme", "-a", "alice", "-m", "Initial")
		env.runStdin("version 2", "write", "docs/readme", "-a", "bob", "-m", "Update")

		out := env.run("history", "docs/readme")
		env.contains(out, "v1")
		env.contains(out, "v2")
		env.contains(out, "alice")
		env.contains(out, "bob")
	})

	t.Run("with messages", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme", "-m", "First commit")
		env.runStdin("content", "write", "docs/readme", "-m", "Fix typo")
		env.runStdin("content", "write", "docs/readme", "-m", "Add section")

		out := env.run("history", "docs/readme")
		env.contains(out, "First commit")
		env.contains(out, "Fix typo")
		env.contains(out, "Add section")
	})

	t.Run("single version", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("only version", "write", "docs/readme")

		out := env.run("history", "docs/readme")
		env.contains(out, "v1")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("content", "write", "docs/readme", "-a", "alice")

		out := env.run("history", "docs/readme", "-o", "json")
		env.contains(out, `"version"`)
		env.contains(out, `"author"`)
		env.contains(out, "alice")
	})
}

func TestHistory_Limit(t *testing.T) {
	env := newTestEnv(t)
	for range 10 {
		env.runStdin("version", "write", "docs/readme")
	}

	out := env.run("history", "docs/readme", "-n", "3")
	env.contains(out, "v10")
	env.contains(out, "v9")
	env.contains(out, "v8")
}

func TestHistory_WithDiff(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("line one", "write", "docs/readme")
	env.runStdin("line one\nline two", "write", "docs/readme")

	out := env.run("history", "docs/readme", "-d")
	env.contains(out, "+")
	env.contains(out, "line two")
}

func TestHistory_NotFound(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.runErr("history", "docs/nonexistent")
	if err == nil {
		t.Error("History(nonexistent) = nil, want error")
	}
}

func TestHistory_Deleted(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("v1", "write", "docs/readme")
	env.runStdin("v2", "write", "docs/readme")
	env.run("rm", "docs/readme")

	t.Run("without flag fails", func(t *testing.T) {
		_, err := env.runErr("history", "docs/readme")
		if err == nil {
			t.Error("History(deleted) = nil, want error")
		}
	})

	t.Run("with flag succeeds", func(t *testing.T) {
		out := env.run("history", "docs/readme", "--deleted")
		env.contains(out, "v1")
		env.contains(out, "v2")
	})
}

func TestHistory_NegativeLimit(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("content", "write", "docs/readme")

	_, err := env.runErr("history", "docs/readme", "-n", "-1")
	if err == nil {
		t.Error("History(-n -1) = nil, want error")
	}
}
