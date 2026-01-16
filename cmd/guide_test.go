package cmd

import "testing"

func TestGuide(t *testing.T) {
	t.Run("main guide", func(t *testing.T) {
		env := newTestEnv(t)

		out := env.run("guide")
		env.contains(out, "LLMD Guide")
		env.contains(out, "Quick Start")
		env.contains(out, "Commands")
	})

	t.Run("lists available on not found", func(t *testing.T) {
		env := newTestEnv(t)

		out, _ := env.runErr("guide", "nonexistent")
		env.contains(out, "Available:")
	})
}

func TestGuide_Commands(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		contain string
	}{
		{"write", "write", "llmd write"},
		{"init", "init", "llmd init"},
		{"export", "export", "llmd export"},
		{"import", "import", "llmd import"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)

			out := env.run("guide", tc.topic)
			env.contains(out, tc.contain)
		})
	}
}

func TestGuide_NotFound(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.runErr("guide", "nonexistent")
	if err == nil {
		t.Error("Guide(nonexistent) = nil, want error")
	}
}
