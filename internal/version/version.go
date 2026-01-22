// Package version provides build version information for llmd.
package version

import (
	"fmt"
	"runtime"
	"time"
)

// Build information, set via -ldflags at build time.
var (
	Tag       = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

// Print outputs version information to stdout.
func Print() {
	fmt.Printf("Build Tag:    %s\n", Tag)
	fmt.Printf("Build Commit: %s\n", Commit)
	fmt.Printf("Build Time:   %s\n", formatBuildTime(BuildTime))
	fmt.Printf("Go Version:   %s\n", GoVersion)
	fmt.Printf("Platform:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// formatBuildTime converts the build timestamp to a readable format.
func formatBuildTime(s string) string {
	t, err := time.Parse("2006-01-02_15:04:05_UTC", s)
	if err != nil {
		return s
	}
	return t.Format("2006/01/02 15:04:05 UTC")
}
