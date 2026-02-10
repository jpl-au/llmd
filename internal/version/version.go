// Package version provides build version information for llmd.
// Variables are set at build time via ldflags. Use the build tool in
// llmd-build with a YAML config file for repeatable builds.
//
// Example:
//
//	go build -ldflags="-X github.com/jpl-au/llmd/internal/version.Version=v1.0.0 \
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
	Version   = "dev"     // Version tag (e.g., "v1.0.0")
	GitCommit = "unknown" // Short git commit hash
	BuildTime = "unknown" // RFC3339 build timestamp
)

// Info holds structured version information.
type Info struct {
	BuildTag  string `json:"build_tag"`  // Version tag (e.g., "v1.0.0" or "dev")
	BuildTime string `json:"build_time"` // RFC3339 build timestamp
	GitCommit string `json:"git_commit"` // Short git commit hash
	GoVersion string `json:"go_version"` // Go runtime version
	Platform  string `json:"platform"`   // OS and architecture (e.g., "darwin arm64")
}

// Get returns the current version information.
func Get() Info {
	return Info{
		BuildTag:  Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string suitable for display.
func (i Info) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Build Tag:    %s\n", i.BuildTag)
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
