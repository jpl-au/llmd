package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// capture swaps *stream (os.Stdout or os.Stderr) for a pipe while fn
// runs and returns what fn wrote.
func capture(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := *stream
	*stream = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	*stream = orig
	return <-done
}

// feedStdin replaces os.Stdin with a pipe holding content for the
// duration of fn, so run reads it as piped input.
func feedStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, content); err != nil {
		t.Fatal(err)
	}
	w.Close()
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()
	fn()
}

// TestParseGlobal covers the global flags, in both positions (before
// and after the command), their stripping from the command's own
// arguments, and the errors for a flag without its value.
func TestParseGlobal(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    globalFlags
		wantErr string
	}{
		{name: "no arguments", args: nil, want: globalFlags{}},
		{name: "command only", args: []string{"ls"}, want: globalFlags{Cmd: "ls", Args: []string{}}},
		{
			name: "flags before the command",
			args: []string{"--json", "--verbose", "--author", "alice", "--db", "x.db", "cat", "docs/a"},
			want: globalFlags{JSON: true, Verbose: true, Author: "alice", DB: "x.db", Cmd: "cat", Args: []string{"docs/a"}},
		},
		{
			name: "flags after the command are stripped",
			args: []string{"cat", "docs/a", "--json", "--author=bob", "--verbose", "-h"},
			want: globalFlags{JSON: true, Verbose: true, Author: "bob", Help: true, Cmd: "cat", Args: []string{"docs/a"}},
		},
		{
			name: "author after the command with a separate value",
			args: []string{"write", "--author", "carol", "docs/a"},
			want: globalFlags{Author: "carol", Cmd: "write", Args: []string{"docs/a"}},
		},
		{
			name: "command flags pass through",
			args: []string{"cat", "--limit", "3", "-n", "docs/a"},
			want: globalFlags{Cmd: "cat", Args: []string{"--limit", "3", "-n", "docs/a"}},
		},
		{
			name: "help before the command",
			args: []string{"--help", "cat"},
			want: globalFlags{Help: true, Cmd: "cat", Args: []string{}},
		},
		{
			name: "equals form of author",
			args: []string{"--author=dave", "ls"},
			want: globalFlags{Author: "dave", Cmd: "ls", Args: []string{}},
		},
		{
			// A leading flag the global parser does not know is left
			// for the command, which then rejects it.
			name: "unknown leading flag is ignored by the global parser",
			args: []string{"--bogus", "ls"},
			want: globalFlags{Cmd: "ls", Args: []string{}},
		},
		{name: "db without a value", args: []string{"--db"}, wantErr: "--db requires a path"},
		{name: "author without a value", args: []string{"--author"}, wantErr: "--author requires a name"},
		{
			// After the command, a trailing --author with no value is
			// not consumed and reaches the command as an argument.
			name: "trailing author after the command is left alone",
			args: []string{"ls", "--author"},
			want: globalFlags{Cmd: "ls", Args: []string{"--author"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGlobal(tt.args)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("parseGlobal(%v) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGlobal(%v) error = %v", tt.args, err)
			}
			if !slices.Equal(got.Args, tt.want.Args) {
				t.Errorf("parseGlobal(%v) Args = %q, want %q", tt.args, got.Args, tt.want.Args)
			}
			got.Args, tt.want.Args = nil, nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseGlobal(%v) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

// TestDisplay covers how each response type reaches stdout, with and
// without --json. Stdout is a pipe here, so markdown is emitted raw.
func TestDisplay(t *testing.T) {
	tests := []struct {
		name     string
		resp     sdk.Response
		jsonOut  bool
		wantOut  string // exact stdout, or "" for none
		wantJSON any    // decoded stdout when non-nil
	}{
		{name: "text", resp: sdk.Text("hello"), wantOut: "hello\n"},
		{name: "empty text prints nothing", resp: sdk.Text(""), wantOut: ""},
		{name: "text ignores json", resp: sdk.Text("hello"), jsonOut: true, wantOut: "hello\n"},
		{name: "markdown raw on a pipe", resp: sdk.Markdown{Text: "# Title", Data: "x"}, wantOut: "# Title\n"},
		{name: "markdown data as json", resp: sdk.Markdown{Text: "# Title", Data: map[string]any{"a": 1.0}}, jsonOut: true, wantJSON: map[string]any{"a": 1.0}},
		{name: "markdown without data falls back to text under json", resp: sdk.Markdown{Text: "# Title"}, jsonOut: true, wantOut: "# Title\n"},
		{name: "result text", resp: sdk.Result{Text: "done", Data: []string{"a"}}, wantOut: "done\n"},
		{name: "result data as json", resp: sdk.Result{Text: "done", Data: []any{"a"}}, jsonOut: true, wantJSON: []any{"a"}},
		{name: "result without text prints nothing", resp: sdk.Result{Data: []string{"a"}}, wantOut: ""},
		{name: "data is always json", resp: sdk.Data{V: map[string]any{"k": "v"}}, wantJSON: map[string]any{"k": "v"}},
		{name: "nil response prints nothing", resp: nil, wantOut: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			out := capture(t, &os.Stdout, func() { code = display(tt.resp, tt.jsonOut) })
			if code != 0 {
				t.Errorf("display(%+v) exit code = %d, want 0", tt.resp, code)
			}
			if tt.wantJSON != nil {
				var got any
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatalf("display(%+v) stdout is not JSON: %v\n%s", tt.resp, err, out)
				}
				if !equalJSON(got, tt.wantJSON) {
					t.Errorf("display(%+v) stdout = %v, want %v", tt.resp, got, tt.wantJSON)
				}
				return
			}
			if out != tt.wantOut {
				t.Errorf("display(%+v) stdout = %q, want %q", tt.resp, out, tt.wantOut)
			}
		})
	}
}

func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// TestErrorf covers the plain and JSON error forms on stderr and the
// exit code.
func TestErrorf(t *testing.T) {
	tests := []struct {
		name    string
		jsonOut bool
		want    string
	}{
		{"plain", false, "error: boom: 42\n"},
		{"json", true, `{"error":"boom: 42"}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			out := capture(t, &os.Stderr, func() { code = errorf(tt.jsonOut, "boom: %d", 42) })
			if code != 1 {
				t.Errorf("errorf exit code = %d, want 1", code)
			}
			if out != tt.want {
				t.Errorf("errorf stderr = %q, want %q", out, tt.want)
			}
		})
	}
}

// TestInitLog covers the level chosen from --verbose and the config,
// and that --json switches to the JSON handler.
func TestInitLog(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	tests := []struct {
		name      string
		level     string
		format    string
		jsonOut   bool
		verbose   bool
		wantLevel slog.Level
		wantJSON  bool
	}{
		{name: "default is warn", wantLevel: slog.LevelWarn},
		{name: "verbose is debug", verbose: true, wantLevel: slog.LevelDebug},
		{name: "config debug", level: "debug", wantLevel: slog.LevelDebug},
		{name: "config info", level: "info", wantLevel: slog.LevelInfo},
		{name: "config error", level: "error", wantLevel: slog.LevelError},
		{name: "verbose beats config", level: "error", verbose: true, wantLevel: slog.LevelDebug},
		{name: "config json format", format: "json", wantLevel: slog.LevelWarn, wantJSON: true},
		{name: "json output implies json logs", jsonOut: true, wantLevel: slog.LevelWarn, wantJSON: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{}
			cfg.Log.Level = tt.level
			cfg.Log.Format = tt.format
			out := capture(t, &os.Stderr, func() {
				initLog(cfg, tt.jsonOut, tt.verbose)
				slog.Log(t.Context(), tt.wantLevel, "probe")
				slog.Log(t.Context(), tt.wantLevel-4, "below")
			})
			if !strings.Contains(out, "probe") {
				t.Errorf("a message at %v was not logged: %q", tt.wantLevel, out)
			}
			if strings.Contains(out, "below") {
				t.Errorf("a message below %v was logged: %q", tt.wantLevel, out)
			}
			if isJSON := strings.HasPrefix(out, "{"); isJSON != tt.wantJSON {
				t.Errorf("log format json = %v, want %v: %q", isJSON, tt.wantJSON, out)
			}
		})
	}
}

// TestHelp covers the root listing (every advertised command exists
// and shows one line) and per-command help (usage, flags with their
// types, global flags).
func TestHelp(t *testing.T) {
	h := host.New()
	out := capture(t, &os.Stdout, func() { printHelp(h) })
	cmds := h.Commands()
	for _, g := range helpGroups {
		if !strings.Contains(out, g.label+":") {
			t.Errorf("root help lacks group %q", g.label)
		}
		for _, name := range g.names {
			if _, ok := cmds[name]; !ok {
				t.Errorf("root help advertises %q, which is not a command", name)
				continue
			}
			if !strings.Contains(out, "  "+name) {
				t.Errorf("root help lacks command %q", name)
			}
		}
	}
	if strings.Count(out, "\n\n\n") > 0 {
		t.Errorf("root help has a blank gap:\n%s", out)
	}

	cat := cmds["cat"]
	out = capture(t, &os.Stdout, func() { printCmdHelp(cat) })
	for _, want := range []string{"cat - ", "Usage:\n  llmd cat", "Flags:", "--version <int>", "--offset <int>", "    --n ", "Global Flags:", "--author <name>"} {
		if !strings.Contains(out, want) {
			t.Errorf("cat help lacks %q:\n%s", want, out)
		}
	}
}

// TestRun drives the binary's entry point end to end in a temporary
// project: help, version, init, a write from piped stdin, a read, the
// help shown when a command lacks its arguments, and the failures.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
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
	origLog := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origLog) })

	steps := []struct {
		name     string
		args     []string
		stdin    string
		wantCode int
		wantOut  []string // substrings of stdout
		wantErr  []string // substrings of stderr
	}{
		{name: "no arguments shows help", args: nil, wantOut: []string{"Usage:", "Reading:"}},
		{name: "version", args: []string{"version"}, wantOut: []string{"Build Tag:", "Go Version:"}},
		{name: "command help", args: []string{"cat", "--help"}, wantOut: []string{"Usage:\n  llmd cat"}},
		{name: "help for an unknown command", args: []string{"nope", "--help"}, wantCode: 1, wantErr: []string{"unknown command: nope"}},
		{name: "bad global flag", args: []string{"--db"}, wantCode: 1, wantErr: []string{"--db requires a path"}},
		{name: "store missing", args: []string{"ls"}, wantCode: 1, wantErr: []string{"store not found"}},
		{name: "init", args: []string{"init"}, wantOut: []string{"Initialised llmd store"}},
		{name: "write from stdin", args: []string{"--author", "alice", "write", "docs/a"}, stdin: "# A\n", wantOut: []string{"Wrote docs/a"}},
		{name: "cat", args: []string{"cat", "docs/a"}, wantOut: []string{"# A"}},
		{name: "cat as json", args: []string{"--json", "cat", "docs/a"}, wantOut: []string{`"# A\n"`}},
		{name: "missing argument shows command help", args: []string{"cat"}, wantCode: 1, wantOut: []string{"Usage:\n  llmd cat"}},
		{name: "write without author", args: []string{"write", "docs/b"}, stdin: "x", wantCode: 1, wantOut: []string{"Usage:\n  llmd write"}},
		{name: "not found", args: []string{"cat", "docs/missing"}, wantCode: 1, wantErr: []string{"not found"}},
		{name: "json error", args: []string{"--json", "cat", "docs/missing"}, wantCode: 1, wantErr: []string{`{"error":"`}},
	}

	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			var code int
			var stderr string
			stdout := capture(t, &os.Stdout, func() {
				stderr = capture(t, &os.Stderr, func() {
					if st.stdin != "" {
						feedStdin(t, st.stdin, func() { code = run(st.args) })
					} else {
						code = run(st.args)
					}
				})
			})
			if code != st.wantCode {
				t.Errorf("run(%v) = %d, want %d\nstdout: %s\nstderr: %s", st.args, code, st.wantCode, stdout, stderr)
			}
			for _, want := range st.wantOut {
				if !strings.Contains(stdout, want) {
					t.Errorf("run(%v) stdout = %q, want to contain %q", st.args, stdout, want)
				}
			}
			for _, want := range st.wantErr {
				if !strings.Contains(stderr, want) {
					t.Errorf("run(%v) stderr = %q, want to contain %q", st.args, stderr, want)
				}
			}
		})
	}
}
