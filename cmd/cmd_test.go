package cmd

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/config"
	"github.com/jpl-au/llmd/host"
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
			// An unknown flag before the command is refused, not
			// dropped: it once ran ls as if it were not there.
			name:    "unknown leading flag is refused",
			args:    []string{"--bogus", "ls"},
			wantErr: "llmd: unknown flag --bogus. The global flags are --author, --json, --db, --verbose and --help.",
		},
		{name: "db without a value", args: []string{"--db"}, wantErr: "llmd: --db needs a path."},
		{name: "author without a value", args: []string{"--author"}, wantErr: "llmd: --author needs a name."},
		{name: "trailing author without a value", args: []string{"ls", "--author"}, wantErr: "llmd: --author needs a name."},
		{name: "db with an empty inline value", args: []string{"--db=", "ls"}, wantErr: "llmd: --db needs a path."},
		{
			// --db=x was once ignored, so the command read the
			// default store.
			name: "db inline before the command",
			args: []string{"--db=docs", "cat", "p"},
			want: globalFlags{DB: "docs", Cmd: "cat", Args: []string{"p"}},
		},
		{
			// --db after the command once reached the command and
			// was refused as unknown.
			name: "db after the command",
			args: []string{"cat", "p", "--db", "docs"},
			want: globalFlags{DB: "docs", Cmd: "cat", Args: []string{"p"}},
		},
		{
			name: "db inline after the command",
			args: []string{"cat", "--db=docs", "p"},
			want: globalFlags{DB: "docs", Cmd: "cat", Args: []string{"p"}},
		},
		{
			// After --, everything belongs to the command.
			name: "double dash ends global flags",
			args: []string{"write", "p", "--", "--json"},
			want: globalFlags{Cmd: "write", Args: []string{"p", "--", "--json"}},
		},
		{name: "version before the command", args: []string{"--version"}, want: globalFlags{Version: true, Args: nil}},
		{name: "short version", args: []string{"-v"}, want: globalFlags{Version: true}},
		{
			// After the command, --version is the command's flag.
			name: "version after the command belongs to it",
			args: []string{"cat", "--version", "3", "p"},
			want: globalFlags{Cmd: "cat", Args: []string{"--version", "3", "p"}},
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
		// Under --json every response is JSON: text has no structured
		// form, so it comes out as {"text": ...}.
		{name: "text as json", resp: sdk.Text("hello"), jsonOut: true, wantJSON: map[string]any{"text": "hello"}},
		{name: "markdown raw on a pipe", resp: sdk.Markdown{Text: "# Title", Data: "x"}, wantOut: "# Title\n"},
		{name: "markdown data as json", resp: sdk.Markdown{Text: "# Title", Data: map[string]any{"a": 1.0}}, jsonOut: true, wantJSON: map[string]any{"a": 1.0}},
		{name: "markdown without data as json", resp: sdk.Markdown{Text: "# Title"}, jsonOut: true, wantJSON: map[string]any{"text": "# Title"}},
		{name: "result without data as json", resp: sdk.Result{Text: "done"}, jsonOut: true, wantJSON: map[string]any{"text": "done"}},
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
		{"plain", false, "boom: 42\n"},
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
	t.Run("an added extension's commands are listed under its name", func(t *testing.T) {
		builtin := helpExtension{name: "cli", commands: []string{"cat", "stat"}}
		added := helpExtension{name: "gdd", commands: []string{"open"}}
		got := extensionGroups([]sdk.Extension{builtin, added})
		if len(got) != 2 || got[0].label != "cli" || !slices.Equal(got[0].names, []string{"stat"}) || got[1].label != "gdd" || !slices.Equal(got[1].names, []string{"open"}) {
			t.Errorf("extensionGroups() = %+v, want cli [stat] and gdd [open]", got)
		}
	})

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
	// Every command is advertised: audit, queue, rule and graph were
	// once left out of the listing although the guide lists them.
	listed := map[string]bool{}
	for _, g := range helpGroups {
		for _, name := range g.names {
			listed[name] = true
		}
	}
	for name := range cmds {
		if !listed[name] {
			t.Errorf("root help does not list the command %q", name)
		}
	}
	for _, flag := range []string{"--author", "--db", "--json", "--verbose", "--help", "--version"} {
		if !strings.Contains(out, flag) {
			t.Errorf("root help does not list the global flag %s", flag)
		}
	}

	cat := cmds["cat"]
	t.Run("terminal form", func(t *testing.T) {
		out := cmdHelp(cat, true)
		for _, want := range []string{"cat - " + cat.Desc, "Usage:\n  llmd cat", "Flags:", "--version <int>", "--offset <int>", "    --n ", "Global Flags:", "--author <name>"} {
			if !strings.Contains(out, want) {
				t.Errorf("cat help lacks %q:\n%s", want, out)
			}
		}
	})
	t.Run("piped form is the agent help", func(t *testing.T) {
		out := cmdHelp(cat, false)
		if !strings.HasPrefix(out, cat.AgentHelp()+"\n") {
			t.Errorf("piped cat help = %q, want to start with AgentHelp", out)
		}
		for _, want := range []string{"Details: guide cat", "--author <name>", "--json"} {
			if !strings.Contains(out, want) {
				t.Errorf("piped cat help lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "Global Flags:") || strings.Contains(out, cat.Desc) {
			t.Errorf("piped cat help carries the terminal form:\n%s", out)
		}
	})
	// printCmdHelp picks the form from stdout; under test that is a pipe.
	out = capture(t, &os.Stdout, func() { printCmdHelp(cat) })
	if out != cmdHelp(cat, false) {
		t.Errorf("printCmdHelp to a pipe = %q, want the piped form", out)
	}
}

// TestRun drives the binary's entry point end to end in a temporary
// project: help, version, init, a write from piped stdin, a read, the
// help shown when a command lacks its arguments, and the failures.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	host.TestHome(t)
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
		{name: "command help", args: []string{"cat", "--help"}, wantOut: []string{"Usage: llmd cat", "Details: guide cat"}},
		{name: "help for an unknown command", args: []string{"nope", "--help"}, wantCode: 1, wantErr: []string{"llmd: unknown command nope."}},
		{name: "bad global flag", args: []string{"--db"}, wantCode: 1, wantErr: []string{"llmd: --db needs a path."}},
		{name: "store missing", args: []string{"ls"}, wantCode: 1, wantErr: []string{"ls: there is no store at " + filepath.Join(".llmd", "llmd.db") + ". Run llmd init to create one."}},
		{name: "init", args: []string{"init"}, wantOut: []string{"Initialised llmd store"}},
		{name: "write from stdin", args: []string{"--author", "alice", "write", "docs/a"}, stdin: "# A\n", wantOut: []string{"Wrote docs/a"}},
		{name: "cat", args: []string{"cat", "docs/a"}, wantOut: []string{"# A"}},
		{name: "cat as json", args: []string{"--json", "cat", "docs/a"}, wantOut: []string{`"# A\n"`}},
		{name: "missing argument shows command help", args: []string{"cat"}, wantCode: 1, wantOut: []string{"Usage: llmd cat"}},
		{name: "write without author", args: []string{"write", "docs/b"}, stdin: "x", wantCode: 1, wantOut: []string{"Usage: llmd write", "--author <name>"}},
		{name: "not found", args: []string{"cat", "docs/missing"}, wantCode: 1, wantErr: []string{"docs/missing does not exist."}},
		{name: "json error", args: []string{"--json", "cat", "docs/missing"}, wantCode: 1, wantErr: []string{`{"error":"`}},
	}

	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			var code int
			var stderr string
			stdout := capture(t, &os.Stdout, func() {
				stderr = capture(t, &os.Stderr, func() {
					if st.stdin != "" {
						feedStdin(t, st.stdin, func() { code = Execute(st.args) })
					} else {
						code = Execute(st.args)
					}
				})
			})
			if code != st.wantCode {
				t.Errorf("Execute(%v) = %d, want %d\nstdout: %s\nstderr: %s", st.args, code, st.wantCode, stdout, stderr)
			}
			for _, want := range st.wantOut {
				if !strings.Contains(stdout, want) {
					t.Errorf("Execute(%v) stdout = %q, want to contain %q", st.args, stdout, want)
				}
			}
			for _, want := range st.wantErr {
				if !strings.Contains(stderr, want) {
					t.Errorf("Execute(%v) stderr = %q, want to contain %q", st.args, stderr, want)
				}
			}
		})
	}
}

// helpExtension is an extension with only names, for the help listing.
type helpExtension struct {
	name     string
	commands []string
}

func (e helpExtension) Name() string { return e.name }

func (e helpExtension) Commands() []sdk.Command {
	cmds := make([]sdk.Command, len(e.commands))
	for i, name := range e.commands {
		cmds[i] = sdk.Command{Name: name}
	}
	return cmds
}

func (helpExtension) Exec(sdk.Context, string, []string) (sdk.Response, error) { return nil, nil }
