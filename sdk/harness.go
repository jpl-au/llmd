package sdk

import "context"

// Harness runs an agent program for one run on a task. llmd's own harness, named llmd, runs the command in the agent's config.json. A build of llmd that brings its own harness registers it with harness.Register from the ai/harness package. An agent runs under the harness the store names for it, unless agent.harness in the configuration file names the harness this computer runs.
type Harness interface {
	// Name is the name the store and the configuration file use for the harness.
	Name() string

	// Run runs the program to its end and returns what the run came to. ctx ends when the run is stopped, and the harness then stops its program. A program that runs and fails is not an error: its exit code is in the result. An error states that the program could not be run.
	Run(ctx context.Context, spec RunSpec) (*RunResult, error)
}

// RunSpec is one run of an agent on a task.
type RunSpec struct {
	// Agent is the name of the agent, such as claude-code.
	Agent string `json:"agent"`

	// Command and Args are the program and its arguments from the agent's config.json. An argument may hold {{.Prompt}}, which takes the prompt. A harness that knows the program in a way of its own may leave them unused.
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`

	// Prompt is what the agent is asked in a new session.
	Prompt string `json:"prompt"`

	// Resume carries on an earlier session of the same agent. Nil starts a new session. A harness that cannot carry a session on runs Prompt in a new one.
	Resume *Resume `json:"resume,omitempty"`

	// Settings is a file of settings for the program to read for this run, such as its permissions and hooks. Empty means the settings are already where the program reads them.
	Settings string `json:"settings,omitempty"`

	// Budget is the most the run may spend, in US dollars. Zero sets no limit.
	Budget float64 `json:"budget,omitempty"`

	// Dir is the folder the agent works in.
	Dir string `json:"dir"`

	// Log is the file the harness writes the program's output to. When the run fails, llmd records the end of the file as an audit note on the task.
	Log string `json:"log"`
}

// Resume is an earlier session to carry on, and what the agent is told in it.
type Resume struct {
	Session string `json:"session"`
	Prompt  string `json:"prompt"`
}

// RunResult is what a run came to. A field the harness cannot state is zero or nil.
type RunResult struct {
	// ExitCode is the exit code of the program. Zero is success.
	ExitCode int

	// Session is the session the run used, which a later run can carry on.
	Session string

	// Model is the model that answered.
	Model string

	// Cost is what the run spent, in US dollars.
	Cost *float64

	// InputTokens and OutputTokens are the tokens the run read and wrote.
	InputTokens  *int
	OutputTokens *int
}
