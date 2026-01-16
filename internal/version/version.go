// Package version provides build version information for llmd.
// Variables are set at build time via ldflags. Use the build tool in
// llmd-build with a YAML config file for repeatable builds.
//
// Standard edition build (Edition and BaseVersion are not set):
//
//	go build -ldflags="-X github.com/jpl-au/llmd/internal/version.Version=v1.0.0 \
//	  -X github.com/jpl-au/llmd/internal/version.GitCommit=abc123 \
//	  -X github.com/jpl-au/llmd/internal/version.BuildTime=2024-01-15T10:30:00Z"
//
// Pro edition build (Edition and BaseVersion are set):
//
//	go build -ldflags="-X github.com/jpl-au/llmd/internal/version.Edition=pro \
//	  -X github.com/jpl-au/llmd/internal/version.Version=v1.0.0 \
//	  -X github.com/jpl-au/llmd/internal/version.BaseVersion=v0.9.0 \
//	  -X github.com/jpl-au/llmd/internal/version.GitCommit=abc123 \
//	  -X github.com/jpl-au/llmd/internal/version.BuildTime=2024-01-15T10:30:00Z"
package version

import (
	"fmt"
	"runtime"
	"strings"
)

// Build information. Set via ldflags at build time.
var (
	Edition     = ""        // Only set for non-standard editions (e.g., "pro")
	Version     = "dev"     // Version tag (e.g., "v1.0.0")
	BaseVersion = ""        // Base llmd version (pro only)
	GitCommit   = "unknown" // Short git commit hash
	BuildTime   = "unknown" // RFC3339 build timestamp
)

// Info holds structured version information.
type Info struct {
	Edition     string `json:"edition"`      // Edition label (empty for standard, "pro" for pro)
	BuildTag    string `json:"build_tag"`    // Version tag (e.g., "v1.0.0" or "dev")
	BaseVersion string `json:"base_version"` // Base llmd version (pro only)
	BuildTime   string `json:"build_time"`   // RFC3339 build timestamp
	GitCommit   string `json:"git_commit"`   // Short git commit hash
	GoVersion   string `json:"go_version"`   // Go runtime version
	Platform    string `json:"platform"`     // OS and architecture (e.g., "darwin arm64")
}

// Get returns the current version information.
func Get() Info {
	return Info{
		Edition:     Edition,
		BuildTag:    Version,
		BaseVersion: BaseVersion,
		BuildTime:   BuildTime,
		GitCommit:   GitCommit,
		GoVersion:   runtime.Version(),
		Platform:    fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string suitable for display.
func (i Info) String() string {
	var b strings.Builder
	if i.Edition != "" && i.Edition != "standard" {
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
	return b.String()
}

// Short returns just the version string (e.g., "v1.0.0" or "dev").
func Short() string {
	return Version
}
