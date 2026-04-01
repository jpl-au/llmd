package assets

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// RunStats holds execution metrics extracted from an agent's output.
// All fields are nullable - agents report what they can.
type RunStats struct {
	MonetaryCost *float64
	InputTokens  *int
	OutputTokens *int
	Model        string
	SessionID    string
}

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

	// Stats extracts execution metrics from the agent's output log.
	// Returns nil when no stats are available.
	Stats(logPath string) (*RunStats, error)

	// ResumeArgs returns CLI arguments that restore the agent's prior
	// conversation context, so it can continue work without rebuilding
	// its understanding of the task from scratch. For example, Claude
	// Code uses "--resume <session-id>". Returns nil when the platform
	// has no session resumption mechanism, in which case Spawn falls
	// back to a fresh prompt.
	ResumeArgs(sessionID string) []string
}

// Platform returns the Platform for the named agent. Unknown agents
// get a no-op implementation.
func (*agentAssets) Platform(name string) Platform {
	switch {
	case strings.Contains(name, "claude"):
		return claude{}
	case strings.Contains(name, "gemini"):
		return gemini{}
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

func (claude) ResumeArgs(sessionID string) []string {
	return []string{"--resume", sessionID}
}

// Stats reads the agent log and extracts metrics from Claude Code's
// JSON output (--output-format json).
func (claude) Stats(logPath string) (*RunStats, error) {
	data, err := LastJSON(logPath)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}

	var output struct {
		TotalCost *float64 `json:"total_cost_usd"`
		SessionID string   `json:"session_id"`
		Usage     struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		ModelUsage map[string]json.RawMessage `json:"modelUsage"`
	}
	if err := json.Unmarshal([]byte(data), &output); err != nil {
		slog.Debug("parsing claude JSON output", "error", err)
		return nil, nil
	}

	stats := &RunStats{
		MonetaryCost: output.TotalCost,
		SessionID:    output.SessionID,
	}
	if output.Usage.InputTokens > 0 {
		stats.InputTokens = &output.Usage.InputTokens
	}
	if output.Usage.OutputTokens > 0 {
		stats.OutputTokens = &output.Usage.OutputTokens
	}
	// Model name is the first key in modelUsage.
	for name := range output.ModelUsage {
		stats.Model = name
		break
	}
	return stats, nil
}

// gemini is the Platform for Gemini CLI.
type gemini struct{}

func (gemini) SettingsPath() string        { return "" }
func (gemini) BudgetArgs(float64) []string { return nil }
func (gemini) ResumeArgs(string) []string  { return nil }

// Stats reads the agent log and extracts metrics from Gemini CLI's
// JSON output (--output-format json).
func (gemini) Stats(logPath string) (*RunStats, error) {
	data, err := LastJSON(logPath)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}

	var output struct {
		Stats struct {
			Models map[string]struct {
				Tokens struct {
					Input      int `json:"input"`
					Candidates int `json:"candidates"`
				} `json:"tokens"`
			} `json:"models"`
		} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(data), &output); err != nil {
		slog.Debug("parsing gemini JSON output", "error", err)
		return nil, nil
	}

	stats := &RunStats{}
	for name, m := range output.Stats.Models {
		stats.Model = name
		if m.Tokens.Input > 0 {
			stats.InputTokens = &m.Tokens.Input
		}
		if m.Tokens.Candidates > 0 {
			stats.OutputTokens = &m.Tokens.Candidates
		}
		break
	}

	if stats.Model == "" {
		return nil, nil
	}
	return stats, nil
}

// generic is the no-op Platform for unknown agents.
type generic struct{}

func (generic) SettingsPath() string            { return "" }
func (generic) BudgetArgs(float64) []string     { return nil }
func (generic) ResumeArgs(string) []string      { return nil }
func (generic) Stats(string) (*RunStats, error) { return nil, nil }

// LastJSON scans a file and returns the last line that starts with
// '{'. Agent tools write their JSON result as the final output line.
func LastJSON(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening agent log: %w", err)
	}
	defer f.Close()

	var last string
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "{") {
			last = line
		}
	}
	if err := s.Err(); err != nil {
		return "", fmt.Errorf("reading agent log: %w", err)
	}
	return last, nil
}
