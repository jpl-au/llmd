package agents

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
)

//go:embed wrappers/run.sh
var wrapperTemplate string

// WrapperData holds the values substituted into the wrapper script.
type WrapperData struct {
	TaskID     string
	Agent      string
	Role       string // developer, tester, or auditor
	LLMD       string // absolute path to the llmd binary
	URL        string // HTTP API URL (empty if server not configured)
	ProjectDir string // absolute path to the project root (where .llmd/ lives)
	Worktree   string // absolute path to the worktree
	Command    string // agent command (resolved path)
	Args       string // agent arguments as a single string
	OnSuccess  string // column to move to on exit 0 (empty = role default)
	OnFailure  string // column to move to on non-zero exit (empty = "failed")
}

// GenerateWrapper writes the run wrapper script into the worktree
// and returns its path. The wrapper runs the agent and handles task
// lifecycle (moving the task based on exit code, checking for hook
// pre-emption).
func GenerateWrapper(worktree string, data WrapperData) (string, error) {
	tmpl, err := texttemplate.New("wrapper").Parse(wrapperTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing wrapper template: %w", err)
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("executing wrapper template: %w", err)
	}

	path := filepath.Join(worktree, ".llmd-run.sh")
	if err := os.WriteFile(path, []byte(b.String()), 0755); err != nil {
		return "", fmt.Errorf("writing wrapper script: %w", err)
	}

	return path, nil
}
