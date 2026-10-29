package host

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/term"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// isolatedCwd moves the test into an empty directory with an empty
// HOME, so config.Load finds neither a local nor a global config and
// author resolution depends only on the flag and the terminal.
func isolatedCwd(t *testing.T) string {
	t.Helper()
	TestHome(t)
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

// dbPathOf returns the llmd.db path of a host built by TestSetup.
func dbPathOf(h *Host) string {
	return filepath.Join(h.Dir(), ".llmd", "llmd.db")
}

// TestRun covers the CLI entry point: command lookup, opening a store
// for the call (or carrying on without one for storeless and
// streaming commands), author resolution and dispatch. The tests
// are non-interactive, so a configured author never applies and a
// command that needs an author fails without --author.
func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		host     func(t *testing.T) (*Host, string) // host and the --db value
		opts     RunOpts
		wantErr  error
		wantAny  bool   // any error
		wantText string // response text when no error is expected
		skipTTY  bool   // skip when stdout is a terminal (author would come from config)
	}{
		{
			name:    "unknown command",
			host:    func(*testing.T) (*Host, string) { return New(), "" },
			opts:    RunOpts{Cmd: "no-such-command"},
			wantErr: sdk.ErrUnknownCmd,
		},
		{
			name: "storeless command without a store",
			host: func(t *testing.T) (*Host, string) {
				return New(), filepath.Join(t.TempDir(), "missing", "llmd.db")
			},
			opts:     RunOpts{Cmd: "version"},
			wantText: "Build Tag",
		},
		{
			name: "store command without a store fails",
			host: func(t *testing.T) (*Host, string) {
				return New(), filepath.Join(t.TempDir(), "missing", "llmd.db")
			},
			opts:    RunOpts{Cmd: "ls"},
			wantAny: true,
		},
		{
			name: "store command opens the store for the call",
			host: func(t *testing.T) (*Host, string) {
				s := TestSetup(t, TestIsolated)
				writeDoc(t, s.Context(t.Context(), "alice"), "docs/opened", "# Opened")
				return New(), dbPathOf(s)
			},
			opts:     RunOpts{Cmd: "cat", Args: []string{"docs/opened"}},
			wantText: "# Opened",
		},
		{
			name: "host with a store runs directly",
			host: func(t *testing.T) (*Host, string) {
				s := TestSetup(t, TestIsolated)
				writeDoc(t, s.Context(t.Context(), "alice"), "docs/direct", "# Direct")
				return s, ""
			},
			opts:     RunOpts{Cmd: "cat", Args: []string{"docs/direct"}},
			wantText: "# Direct",
		},
		{
			name: "author flag wins",
			host: func(t *testing.T) (*Host, string) {
				isolatedCwd(t)
				if err := os.MkdirAll(".llmd", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(".llmd", "config.yaml"), []byte("author: jake\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return TestSetup(t, TestIsolated), ""
			},
			opts:     RunOpts{Cmd: "write", Args: []string{"docs/by-flag"}, Author: "alice", Stdin: strings.NewReader("# Flag")},
			wantText: "docs/by-flag",
		},
		{
			name: "author required without flag",
			host: func(t *testing.T) (*Host, string) {
				isolatedCwd(t)
				return TestSetup(t, TestIsolated), ""
			},
			opts:    RunOpts{Cmd: "write", Args: []string{"docs/no-author"}, Stdin: strings.NewReader("x")},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name: "configured author does not apply when non-interactive",
			host: func(t *testing.T) (*Host, string) {
				isolatedCwd(t)
				if err := os.MkdirAll(".llmd", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(".llmd", "config.yaml"), []byte("author: jake\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return TestSetup(t, TestIsolated), ""
			},
			opts:    RunOpts{Cmd: "write", Args: []string{"docs/configured"}, Stdin: strings.NewReader("x")},
			wantErr: sdk.ErrMissingArg,
			skipTTY: true,
		},
		{
			name: "command that needs no author runs without one",
			host: func(t *testing.T) (*Host, string) {
				isolatedCwd(t)
				return TestSetup(t, TestIsolated), ""
			},
			opts:     RunOpts{Cmd: "version"},
			wantText: "Build Tag",
		},
		{
			name: "streaming command starts without a store",
			host: func(t *testing.T) (*Host, string) {
				registerTestExtension(t)
				return New(), filepath.Join(t.TempDir(), "missing", "llmd.db")
			},
			opts:     RunOpts{Cmd: "{stream}"},
			wantText: "streamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTTY && term.Interactive() {
				t.Skip("stdout is a terminal: the configured author would apply")
			}
			h, db := tt.host(t)
			opts := tt.opts
			opts.DB = db
			if opts.Cmd == "{stream}" {
				opts.Cmd = testExt.streamCmd()
			}

			resp, err := h.Run(t.Context(), opts)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Run(%s %v) error = %v, want %v", opts.Cmd, opts.Args, err, tt.wantErr)
				}
			case tt.wantAny:
				if err == nil {
					t.Errorf("Run(%s %v) error = nil, want an error", opts.Cmd, opts.Args)
				}
			default:
				if err != nil {
					t.Fatalf("Run(%s %v) error = %v", opts.Cmd, opts.Args, err)
				}
				if got := text(resp); !strings.Contains(got, tt.wantText) {
					t.Errorf("Run(%s %v) = %q, want to contain %q", opts.Cmd, opts.Args, got, tt.wantText)
				}
			}
		})
	}
}

// TestResolveAuthor covers the flag taking precedence and the
// required/optional outcomes with no author configured anywhere.
func TestResolveAuthor(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		required bool
		want     string
		wantErr  error
	}{
		{"flag wins when required", "alice", true, "alice", nil},
		{"flag wins when optional", "alice", false, "alice", nil},
		{"optional without flag is empty", "", false, "", nil},
		{"required without flag fails", "", true, "", sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatedCwd(t)
			h := New()
			got, err := h.resolveAuthor("write", tt.flag, tt.required)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("resolveAuthor(%q, %v) error = %v, want %v", tt.flag, tt.required, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("resolveAuthor(%q, %v) = %q, want %q", tt.flag, tt.required, got, tt.want)
			}
		})
	}
}

// TestCommands checks that the command table lists the compiled
// extension's commands and that the returned copy is detached from
// the host.
func TestCommands(t *testing.T) {
	h := New()
	cmds := h.Commands()

	cat, ok := cmds["cat"]
	if !ok {
		t.Fatalf("Commands() has no %q; names = %v", "cat", keys(cmds))
	}
	if cat.Name != "cat" || cat.NeedsAuthor {
		t.Errorf("Commands()[cat] = %+v, want Name cat without NeedsAuthor", cat)
	}
	if w := cmds["write"]; w == nil || !w.NeedsAuthor {
		t.Errorf("Commands()[write] = %+v, want NeedsAuthor", w)
	}

	cat.Desc = "changed"
	if again := h.Commands()["cat"]; again.Desc == "changed" {
		t.Errorf("Commands() returned a shared Command: mutation leaked into the host")
	}
}

func keys(m map[string]*sdk.Command) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestExtensions checks that the loaded extensions include the CLI
// and that a registered test extension is picked up by later hosts.
func TestExtensions(t *testing.T) {
	ext := registerTestExtension(t)
	h := New()

	var names []string
	for _, e := range h.Extensions() {
		names = append(names, e.Name())
	}
	for _, want := range []string{"cli", ext.Name()} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Extensions() = %v, want to include %q", names, want)
		}
	}
	if _, ok := h.Commands()[ext.streamCmd()]; !ok {
		t.Errorf("Commands() lacks the test extension's %q", ext.streamCmd())
	}
}

// TestSubscribe checks that a callback receives store events and that
// subscribing on a storeless host is a no-op.
func TestSubscribe(t *testing.T) {
	tests := []struct {
		name  string
		host  func(t *testing.T) *Host
		write bool
		want  int // DocumentWritten events received
	}{
		{
			name:  "receives store events",
			host:  func(t *testing.T) *Host { return TestSetup(t, TestIsolated) },
			write: true,
			want:  1,
		},
		{
			name: "no-op without a store",
			host: func(*testing.T) *Host { return New() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.host(t)
			var got []pkgevents.Event
			h.Subscribe(func(e pkgevents.Event) {
				if e.Type == pkgevents.DocumentWritten {
					got = append(got, e)
				}
			})
			if tt.write {
				writeDoc(t, h.Context(t.Context(), "alice"), "docs/sub", "x")
			}
			if len(got) != tt.want {
				t.Fatalf("Subscribe callback received %d written events, want %d", len(got), tt.want)
			}
			if tt.want > 0 && (got[0].Path != "docs/sub" || got[0].Author != "alice" || got[0].Version != 1) {
				t.Errorf("event = %+v, want path docs/sub by alice at version 1", got[0])
			}
		})
	}
}

// TestOpen covers opening an existing store and the missing-store
// error, and TestCloseStoreless that Close is safe without a store.
func TestOpen(t *testing.T) {
	tests := []struct {
		name    string
		path    func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "existing store",
			path: func(t *testing.T) string {
				s := TestSetup(t, TestIsolated)
				return dbPathOf(s)
			},
		},
		{
			name:    "missing store",
			path:    func(t *testing.T) string { return filepath.Join(t.TempDir(), ".llmd", "llmd.db") },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path(t)
			h, err := Open(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Open(%q) error = %v, wantErr %v", path, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got, want := h.Dir(), filepath.Dir(filepath.Dir(path)); got != want {
				t.Errorf("Dir() = %q, want %q", got, want)
			}
			c := h.Context(t.Context(), "alice")
			if c.Documents == nil || c.Tasks == nil || c.Hooks == nil {
				t.Errorf("Context() on an opened host has nil bridges: %+v", c)
			}
			if err := h.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
	}
}

// TestInit covers creating a store through the runtime, with its
// default .gitignore, and the failure for a path that already holds
// one.
func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		path    func(t *testing.T) string
		wantErr bool
	}{
		{"new store", func(t *testing.T) string { return filepath.Join(t.TempDir(), ".llmd", "llmd.db") }, false},
		{"existing store", func(t *testing.T) string { return dbPathOf(TestSetup(t, TestIsolated)) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path(t)
			got, err := New().Init(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Init(%q) error = %v, wantErr %v", path, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != path {
				t.Errorf("Init(%q) = %q, want the same path", path, got)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".gitignore")); err != nil {
				t.Errorf("Init(%q) did not write .llmd/.gitignore: %v", path, err)
			}
			h, err := Open(path)
			if err != nil {
				t.Fatalf("Open after Init error = %v", err)
			}
			if err := h.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
	}
}

func TestCloseStoreless(t *testing.T) {
	h := New()
	if err := h.Close(); err != nil {
		t.Errorf("Close() on a storeless host error = %v, want nil", err)
	}
	if c := h.Context(t.Context(), "alice"); c.Documents != nil || c.Runtime != h {
		t.Errorf("Context() on a storeless host = %+v, want nil bridges and the host as Runtime", c)
	}
}
