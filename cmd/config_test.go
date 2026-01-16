package cmd

import "testing"

func TestConfig(t *testing.T) {
	t.Run("get single key after set", func(t *testing.T) {
		env := newTestEnv(t)

		// Set a value first (init no longer creates config)
		env.run("config", "author.name", "Test User")

		out := env.run("config", "author.name")
		env.contains(out, "Test User")
	})

	t.Run("get all shows defaults", func(t *testing.T) {
		env := newTestEnv(t)

		// Config list should show all keys even without explicit values
		out := env.run("config")
		env.contains(out, "author.name")
		env.contains(out, "author.email")
		env.contains(out, "sync.files")
	})
}

func TestConfig_Set(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"author name", "author.name", "New Name"},
		{"author email", "author.email", "new@example.com"},
		{"sync files true", "sync.files", "true"},
		{"sync files false", "sync.files", "false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)

			env.run("config", tc.key, tc.value)

			out := env.run("config", tc.key)
			env.contains(out, tc.value)
		})
	}
}

func TestConfig_Errors(t *testing.T) {
	t.Run("invalid key", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("config", "invalid.key", "value")
		if err == nil {
			t.Error("Config(invalid key) = nil, want error")
		}
	})

	t.Run("invalid sync value", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("config", "sync.files", "invalid")
		if err == nil {
			t.Error("Config(invalid value) = nil, want error")
		}
	})
}
