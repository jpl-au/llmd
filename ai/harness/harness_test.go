package harness

import (
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/sdk"
)

// TestRegistry checks that llmd's own harness is registered under
// Default and that a second harness of the same name panics.
func TestRegistry(t *testing.T) {
	if h, ok := Lookup(Default); !ok || h.Name() != Default {
		t.Fatalf("Lookup(%q) = %v, %v, want llmd's own harness", Default, h, ok)
	}
	if !slices.Contains(Names(), Default) {
		t.Errorf("Names() = %v, want it to hold %q", Names(), Default)
	}
	if _, ok := Lookup("missing"); ok {
		t.Error("Lookup(missing) found a harness")
	}

	defer func() {
		if recover() == nil {
			t.Error("Register of a second llmd harness did not panic")
		}
	}()
	Register(llmd{})
}

// TestArguments checks the command line llmd's own harness builds: the
// prompt in the agent's arguments, a carried-on session where the
// platform has a way to resume, and the budget and settings flags.
func TestArguments(t *testing.T) {
	args := []string{"-p", "{{.Prompt}}", "--output-format", "json"}
	resume := &sdk.Resume{Session: "s1", Prompt: "fix it"}
	tests := []struct {
		name string
		spec sdk.RunSpec
		want []string
	}{
		{"prompt", sdk.RunSpec{Agent: "claude-code", Args: args, Prompt: "do it"}, []string{"-p", "do it", "--output-format", "json"}},
		// The agent's own arguments stay, so a resumed run writes the same output format and its usage is still read.
		{"resume", sdk.RunSpec{Agent: "claude-code", Args: args, Prompt: "do it", Resume: resume}, []string{"-p", "fix it", "--output-format", "json", "--resume", "s1"}},
		{"no way to resume runs the prompt", sdk.RunSpec{Agent: "gemini", Args: args, Prompt: "do it", Resume: resume}, []string{"-p", "do it", "--output-format", "json"}},
		{"budget", sdk.RunSpec{Agent: "claude-code", Args: args, Prompt: "do it", Budget: 2}, []string{"-p", "do it", "--output-format", "json", "--max-budget-usd=2.00"}},
		{"settings", sdk.RunSpec{Agent: "claude-code", Args: args, Prompt: "do it", Settings: "/runs/t/settings.json"}, []string{"-p", "do it", "--output-format", "json", "--settings", "/runs/t/settings.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := arguments(assets.Agent.Platform(tt.spec.Agent), tt.spec)
			if !slices.Equal(got, tt.want) {
				t.Errorf("arguments() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRun checks that llmd's own harness returns the exit code of the
// command and the usage the platform reads from the log.
func TestRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	line := `{"total_cost_usd":0.42,"session_id":"sess-1","usage":{"input_tokens":10,"output_tokens":20},"modelUsage":{"claude-test":{}}}`
	spec := sdk.RunSpec{
		Agent:   "claude-code",
		Command: "sh",
		Args:    []string{"-c", "echo thinking; echo '" + line + "'; exit 3"},
		Dir:     dir,
		Log:     filepath.Join(dir, "run.log"),
	}

	got, err := llmd{}.Run(t.Context(), spec)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	cost, in, out := 0.42, 10, 20
	want := &sdk.RunResult{ExitCode: 3, Session: "sess-1", Model: "claude-test", Cost: &cost, InputTokens: &in, OutputTokens: &out}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Run() = %+v, want %+v", got, want)
	}
}
