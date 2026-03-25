package assets

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Platform describes platform-specific behaviour for an agent tool
// (Claude Code, Gemini CLI, Aider, Codex, etc.). Common concerns
// like worktree management, prompt assembly, and run tracking stay
// in the caller.
type Platform interface {
	// SettingsPath returns the worktree-relative path where runtime
	// settings should be written. Empty means not supported.
	SettingsPath() string

	// BudgetArgs returns CLI flags for cost control. Nil means the
	// platform has no budget mechanism.
	BudgetArgs(budget float64) []string

	// Cost extracts the monetary spend (USD) from the agent's output
	// log. Returns nil when unavailable.
	Cost(logPath string) (*float64, error)
}

// Platform returns the Platform for the named agent. Unknown agents
// get a no-op implementation.
func (*agentAssets) Platform(name string) Platform {
	switch {
	case strings.Contains(name, "claude"):
		return claude{}
	default:
		return generic{}
	}
}

// claude is the Platform for Claude Code.
type claude struct{}

func (claude) SettingsPath() string {
	return ".claude/settings.json"
}

func (claude) BudgetArgs(budget float64) []string {
	return []string{fmt.Sprintf("--max-budget-usd=%.2f", budget)}
}

// Cost reads the agent log and extracts cost from Claude Code's
// JSON output. Claude Code with --output-format json writes a JSON
// object to stdout containing a total_cost_usd field.
func (claude) Cost(logPath string) (*float64, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("opening agent log: %w", err)
	}
	defer f.Close()

	// Scan for the last JSON object in the log. Claude Code writes
	// its result as a single JSON line to stdout.
	var last string
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "{") {
			last = line
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("reading agent log: %w", err)
	}
	if last == "" {
		return nil, nil
	}

	var output struct {
		TotalCost *float64 `json:"total_cost_usd"`
	}
	if err := json.Unmarshal([]byte(last), &output); err != nil {
		slog.Debug("parsing agent JSON output", "error", err)
		return nil, nil
	}
	return output.TotalCost, nil
}

// generic is the no-op Platform for unknown agents.
type generic struct{}

func (generic) SettingsPath() string                  { return "" }
func (generic) BudgetArgs(float64) []string           { return nil }
func (generic) Cost(string) (*float64, error)         { return nil, nil }
