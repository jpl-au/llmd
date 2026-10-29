package app

import (
	"runtime/debug"
	"testing"
)

// TestFromBuildInfo checks the values a build without ldflags reports:
// go install's module version, and a repository build's commit, time
// and uncommitted changes. An empty version once read "dirty" with no
// commit or time at all.
func TestFromBuildInfo(t *testing.T) {
	tests := []struct {
		name       string
		bi         debug.BuildInfo
		wantTag    string
		wantCommit string
		wantTime   string
	}{
		{
			name:    "go install",
			bi:      debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}},
			wantTag: "v0.3.0",
		},
		{
			name: "repository build with changes",
			bi: debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "624e285070c7abcdef0123"},
				{Key: "vcs.time", Value: "2026-10-27T23:57:04Z"},
				{Key: "vcs.modified", Value: "true"},
			}},
			wantTag:    "dirty",
			wantCommit: "624e285070c7-dirty",
			wantTime:   "2026-10-27T23:57:04Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := Info{BuildTag: "dirty"}
			fromBuildInfo(&info, &tt.bi)
			if info.BuildTag != tt.wantTag || info.GitCommit != tt.wantCommit || info.BuildTime != tt.wantTime {
				t.Errorf("fromBuildInfo() = tag %q commit %q time %q, want %q %q %q",
					info.BuildTag, info.GitCommit, info.BuildTime, tt.wantTag, tt.wantCommit, tt.wantTime)
			}
		})
	}
}
