// Package app provides build-time metadata for the llmd binary.
// Variables are set via ldflags at build time.
//
// Example:
//
//	go build -ldflags="-X github.com/jpl-au/llmd/app.Tag=v1.0.0 \
//	  -X github.com/jpl-au/llmd/app.Commit=abc123 \
//	  -X github.com/jpl-au/llmd/app.Built=2024-01-15T10:30:00Z"
package app

import (
	"fmt"
	"runtime"
	"strings"
)

// Build information injected via ldflags at build time. Defaults
// indicate an unofficial build from source — "dirty" signals the
// binary was not produced by the release build tool.
var (
	Tag         = "dirty" // Version tag (e.g. "v1.0.0")
	Commit      = ""      // Short git commit hash
	Built       = ""      // RFC3339 build timestamp
	Edition     = ""      // Build edition, empty for standard
	BaseVersion = ""      // Base module version when edition is set
	Diagnostics = false   // Set by main when telemetry build tag is active
)

// Info holds structured build information assembled from the injected
// variables and the Go runtime.
type Info struct {
	Edition     string `json:"edition,omitempty"`
	BuildTag    string `json:"build_tag"`
	BaseVersion string `json:"base_version,omitempty"`
	BuildTime   string `json:"build_time"`
	GitCommit   string `json:"git_commit"`
	GoVersion   string `json:"go_version"`
	Platform    string `json:"platform"`
	Diagnostics bool   `json:"diagnostics"`
}

// Version returns the current build information. Runtime values
// (Go version, platform) are always populated; build-time values
// depend on whether ldflags were set.
func Version() Info {
	return Info{
		Edition:     Edition,
		BuildTag:    Tag,
		BaseVersion: BaseVersion,
		BuildTime:   Built,
		GitCommit:   Commit,
		GoVersion:   runtime.Version(),
		Platform:    fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
		Diagnostics: Diagnostics,
	}
}

// String returns a formatted version string suitable for display.
func (i Info) String() string {
	var b strings.Builder
	if i.Edition != "" {
		fmt.Fprintf(&b, "Edition:      %s\n", i.Edition)
	}
	fmt.Fprintf(&b, "Build Tag:    %s\n", i.BuildTag)
	if i.BaseVersion != "" {
		fmt.Fprintf(&b, "Base Version: %s\n", i.BaseVersion)
	}
	fmt.Fprintf(&b, "Build Time:   %s\n", i.BuildTime)
	fmt.Fprintf(&b, "Go Version:   %s\n", i.GoVersion)
	fmt.Fprintf(&b, "Platform:     %s\n", i.Platform)
	fmt.Fprintf(&b, "Git Commit:   %s\n", i.GitCommit)
	if i.Diagnostics {
		fmt.Fprintf(&b, "Diagnostics:  enabled\n")
	}
	return b.String()
}
