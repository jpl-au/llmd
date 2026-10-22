package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// TestExtensions covers the listing of loaded extensions through a
// store-backed and a storeless host: the core "cli" extension is
// always present, names are sorted one per line, and arguments and
// author are ignored.
func TestExtensions(t *testing.T) {
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
				resp := run(t, c, "extensions", nil, tt.args...)
				if _, ok := resp.(sdk.Text); !ok {
					t.Errorf("[%s] extensions %v response type = %T, want sdk.Text", label, tt.args, resp)
				}
				got := text(resp)
				if got == "" {
					t.Fatalf("[%s] extensions %v = empty, want at least cli", label, tt.args)
				}
				lines := strings.Split(got, "\n")
				if !sort.StringsAreSorted(lines) {
					t.Errorf("[%s] extensions %v = %q, want sorted lines", label, tt.args, got)
				}
				found := false
				for _, l := range lines {
					if l == "cli" {
						found = true
					}
					if l == "" || strings.ContainsAny(l, " \t") {
						t.Errorf("[%s] extensions %v has malformed line %q", label, tt.args, l)
					}
				}
				if !found {
					t.Errorf("[%s] extensions %v = %q, want a cli line", label, tt.args, got)
				}
				want := extension.Names()
				sort.Strings(want)
				if strings.Join(want, "\n") != got {
					t.Errorf("[%s] extensions %v = %q, want the registry %v", label, tt.args, got, want)
				}
			}
		})
	}
}
