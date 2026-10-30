package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/assets/platform"
	"github.com/jpl-au/llmd/sdk"
)

// llmd is llmd's own harness. It runs the command in the agent's config.json, writes what the command prints to the log, and reads the usage from the last line of JSON in the log, as each platform states it.
type llmd struct{}

// Name returns Default.
func (llmd) Name() string { return Default }

// Run runs the agent's command to its end. A signal to stop reaches the command through forward, so ctx is not watched.
func (llmd) Run(_ context.Context, spec sdk.RunSpec) (*sdk.RunResult, error) {
	plat := assets.Agent.Platform(spec.Agent)

	cmd := exec.Command(spec.Command, arguments(plat, spec)...)
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	setup(cmd)

	log, err := os.Create(spec.Log)
	if err != nil {
		return nil, fmt.Errorf("creating the agent log: %w", err)
	}
	defer log.Close()
	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting the agent: %w", err)
	}

	// Clear any signal handlers registered by the parent (cmd.Execute's
	// signal.NotifyContext) so that forward() is the sole handler.
	// Without this, an interrupt cancels the parent context and kills
	// the runner before it can manage the child's shutdown.
	signal.Reset(os.Interrupt, syscall.SIGTERM)
	defer forward(cmd)()

	result := &sdk.RunResult{}
	if err := cmd.Wait(); err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return nil, fmt.Errorf("waiting for the agent: %w", err)
		}
		result.ExitCode = exit.ExitCode()
	}

	stats, err := plat.Stats(spec.Log)
	if err != nil {
		slog.Debug("reading agent stats", "log", spec.Log, "error", err)
	}
	if stats != nil {
		result.Session = stats.SessionID
		result.Model = stats.Model
		result.Cost = stats.MonetaryCost
		result.InputTokens = stats.InputTokens
		result.OutputTokens = stats.OutputTokens
	}
	return result, nil
}

// arguments builds the command line of a run from the agent's arguments. A run that carries a session on adds the platform's arguments for that and gives the agent the resume prompt, where the platform has a way to resume, and otherwise starts a new session with the prompt.
func arguments(plat platform.Platform, spec sdk.RunSpec) []string {
	prompt := spec.Prompt
	var session []string
	if spec.Resume != nil {
		if session = plat.ResumeArgs(spec.Resume.Session); session != nil {
			prompt = spec.Resume.Prompt
		}
	}
	args := make([]string, len(spec.Args))
	for i, arg := range spec.Args {
		args[i] = strings.ReplaceAll(arg, "{{.Prompt}}", prompt)
	}
	args = append(args, session...)
	if spec.Budget > 0 {
		args = append(args, plat.BudgetArgs(spec.Budget)...)
	}
	if spec.Settings != "" {
		args = append(args, plat.SettingsArgs(spec.Settings)...)
	}
	return args
}
