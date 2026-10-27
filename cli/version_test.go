package cli

import (
	"runtime"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/app"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// TestVersion covers the version command through a store-backed and
// a storeless host: the text carries every build field with the
// runtime values filled in, the data payload is app.Info, and
// arguments and author are ignored.
func TestVersion(t *testing.T) {
	tests := []struct {
		name   string
		author string
		args   []string
	}{
		{name: "no arguments", author: "alice"},
		{name: "without author", author: ""},
		{name: "arguments are ignored", author: "alice", args: []string{"--bogus", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := newCLI(t, tt.author)
			storeless := host.New().Context(t.Context(), tt.author)
			for label, c := range map[string]sdk.Context{"store": stored, "storeless": storeless} {
				resp := run(t, c, "version", nil, tt.args...)
				got := text(resp)
				for _, want := range []string{
					"Build Tag:",
					"Build Time:",
					"Go Version:   " + runtime.Version(),
					"Platform:     " + runtime.GOOS + " " + runtime.GOARCH,
					"Git Commit:",
				} {
					if !strings.Contains(got, want) {
						t.Errorf("[%s] version %v = %q, want to contain %q", label, tt.args, got, want)
					}
				}
				for line := range strings.SplitSeq(strings.TrimRight(got, "\n"), "\n") {
					if !strings.Contains(line, ":") {
						t.Errorf("[%s] version %v has malformed line %q", label, tt.args, line)
					}
				}
				info, ok := data(resp).(app.Info)
				if !ok {
					t.Fatalf("[%s] version %v Data = %T, want app.Info", label, tt.args, data(resp))
				}
				if info.GoVersion != runtime.Version() || info.Platform != runtime.GOOS+" "+runtime.GOARCH {
					t.Errorf("[%s] version %v Data = %+v, want runtime Go version and platform", label, tt.args, info)
				}
				if info.String() != got {
					t.Errorf("[%s] version %v text = %q, want Data.String() %q", label, tt.args, got, info.String())
				}
				if info.BuildTag == "" {
					t.Errorf("[%s] version %v BuildTag empty, want a tag (dev during development)", label, tt.args)
				}
			}
		})
	}
}
