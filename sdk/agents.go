package sdk

// AgentStore manages agent configurations and agent runs against tasks.
// Agents are external AI tools (Claude Code, Gemini CLI, Aider, etc.)
// that can be spawned to work on tasks in isolated git worktrees.
//
// Configuration is stored in the llmd database (portable with the repo).
// Runs are tracked so consumers (CLI, web UI, MCP) can observe agent
// status and history.
type AgentStore interface {
	// Register adds or updates an agent configuration. If an agent
	// with the same name already exists, it is replaced.
	Register(cfg AgentConfig, author string) error

	// Remove deletes an agent configuration by name.
	Remove(name, author string) error

	// Agent returns a single agent configuration by name.
	Agent(name string) (*AgentConfig, error)

	// Agents returns all registered agent configurations.
	Agents() ([]AgentConfig, error)

	// Spawn creates a git worktree, assembles context from the task's
	// spec, linked documents, and audit history, then starts the agent
	// process. The task must pass Ready() (dependencies satisfied) and
	// must not already have a running agent.
	Spawn(taskKey, agent, author string, opts SpawnOpts) (*AgentRun, error)

	// Run returns the most recent run for a task.
	Run(taskKey string) (*AgentRun, error)

	// Runs returns agent runs matching the filter.
	Runs(opts RunListOpts) ([]*AgentRun, error)

	// Complete records an agent run as finished. Extracts stats
	// (cost, tokens, model) from the agent's output log and
	// cleans up the worktree.
	Complete(taskKey string, exitCode int) error

	// Stop terminates a running agent process and cleans up its
	// worktree.
	Stop(taskKey, author string) error

	// Prompt returns the prompt template for an agent/role combination.
	// Follows the fallback chain: agents/<name>/<role> →
	// agents/default/<role>. Returns the content and the path it was
	// found at.
	Prompt(name, role string) (content, path string, err error)

	// WritePrompt writes a prompt template for an agent/role.
	WritePrompt(name, role, content, author string) error
}

// AgentConfig defines a registered agent that can be spawned for tasks.
type AgentConfig struct {
	// Name identifies this agent configuration (e.g. "claude-code",
	// "gemini", "aider"). Must be unique.
	Name string `json:"name"`

	// Command is the binary name or path to execute (e.g. "claude",
	// "gemini", "/usr/local/bin/aider"). Resolved via PATH if not
	// absolute.
	Command string `json:"command"`

	// Args are default arguments passed to the command. The placeholder
	// {{.Prompt}} is replaced with the assembled context prompt.
	Args []string `json:"args,omitempty"`

	// Role constrains this agent to a specific role (e.g. "developer",
	// "auditor"). Empty means the agent can fill any role.
	Role string `json:"role,omitempty"`

	// MaxBudget is the maximum spend per spawn in USD. Zero means no
	// limit. Passed to the agent via platform-specific flags (e.g.
	// --max-budget-usd for Claude Code).
	MaxBudget float64 `json:"max_budget,omitempty"`
}

// AgentRun represents a single agent execution against a task.
type AgentRun struct {
	// Key is the unique run identifier (9-char base36).
	Key string `json:"key"`

	// TaskKey is the task this agent is working on.
	TaskKey string `json:"task_key"`

	// Agent is the agent configuration name.
	Agent string `json:"agent"`

	// Branch is the git branch the agent is working on.
	Branch string `json:"branch"`

	// Worktree is the filesystem path to the git worktree.
	Worktree string `json:"worktree"`

	// Status is the run lifecycle state.
	Status string `json:"status"`

	// PID is the OS process ID. Zero after the process exits.
	PID int `json:"pid"`

	// ExitCode is the process exit code. -1 while running.
	ExitCode int `json:"exit_code"`

	// MonetaryCost is the cost reported by the agent. Nil when
	// the agent does not report cost.
	MonetaryCost *float64 `json:"monetary_cost,omitempty"`

	// InputTokens is the number of input tokens consumed. Nil
	// when the agent does not report token usage.
	InputTokens *int `json:"input_tokens,omitempty"`

	// OutputTokens is the number of output tokens generated. Nil
	// when the agent does not report token usage.
	OutputTokens *int `json:"output_tokens,omitempty"`

	// Model is the AI model used for this run. Empty when the
	// agent does not report it.
	Model string `json:"model,omitempty"`

	// Author is who initiated the spawn.
	Author string `json:"author"`

	// StartedAt is the Unix timestamp in milliseconds.
	StartedAt int64 `json:"started_at"`

	// StoppedAt is the Unix timestamp in milliseconds. Zero while
	// running.
	StoppedAt int64 `json:"stopped_at,omitempty"`
}

// Agent run status constants.
const (
	AgentRunning   = "running"
	AgentCompleted = "completed"
	AgentFailed    = "failed"
	AgentStopped   = "stopped"
)

// SpawnOpts configures agent spawning.
type SpawnOpts struct {
	// Prompt overrides the auto-assembled context prompt. When empty,
	// context is built from the task's spec, linked documents, audit
	// history, and dependency chain.
	Prompt string

	// MaxBudget overrides the agent config's budget for this spawn.
	// Zero means use the agent config default.
	MaxBudget float64

	// Role overrides the auto-detected role for this spawn. When
	// empty, the role is inferred from the agent config or task
	// status.
	Role string

	// OnSuccess is the column to move to on exit 0. When empty,
	// the wrapper uses role-dependent defaults.
	OnSuccess string

	// OnFailure is the column to move to on non-zero exit. When
	// empty, defaults to "failed".
	OnFailure string
}

// RunListOpts filters agent run queries.
type RunListOpts struct {
	// Status filters by run status (empty = all).
	Status string

	// TaskKey filters by task (empty = all).
	TaskKey string

	// Agent filters by agent name (empty = all).
	Agent string
}
