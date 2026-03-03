// Package plugin loads Yaegi plugins from user directories.
//
// Plugin discovery paths (local overrides global):
//  1. .llmd/plugins/ — project-local plugins
//  2. ~/.llmd/plugins/ — global plugins
//
// Each plugin is a directory containing .go files. If a plugin name exists
// in both local and global, local wins.
package plugin

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"github.com/jpl-au/llmd/sdk"
)

var (
	// ErrNoGoFiles is returned when a plugin directory has no .go files.
	ErrNoGoFiles = errors.New("no .go files found")

	// ErrNotPlugin is returned when New() doesn't return sdk.Plugin.
	ErrNotPlugin = errors.New("New() did not return sdk.Plugin")
)

// Load discovers and loads Yaegi plugins from plugin directories.
func Load() ([]sdk.Plugin, error) {
	dirs := discover()
	if len(dirs) == 0 {
		return nil, nil
	}

	var plugins []sdk.Plugin
	for _, dir := range dirs {
		p, err := load(dir)
		if err != nil {
			slog.Warn("loading plugin", "plugin", filepath.Base(dir), "error", err)
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// discover returns plugin directories, local overriding global.
func discover() []string {
	seen := map[string]bool{}
	var dirs []string

	if local := localDir(); local != "" {
		dirs = append(dirs, scan(local, seen)...)
	}
	if global := globalDir(); global != "" {
		dirs = append(dirs, scan(global, seen)...)
	}

	return dirs
}

// scan returns plugin directories under root, skipping names already in seen.
func scan(root string, seen map[string]bool) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		if seen[name] {
			continue
		}
		dir := filepath.Join(root, name)
		if hasGoFiles(dir) {
			seen[name] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// hasGoFiles reports whether dir contains at least one .go file.
func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// localDir returns .llmd/plugins/ if it exists.
func localDir() string {
	dir := filepath.Join(".llmd", "plugins")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

// globalDir returns ~/.llmd/plugins/ if it exists.
func globalDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".llmd", "plugins")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

// adapter implements sdk.Plugin by calling into a Yaegi interpreter.
// Yaegi-interpreted types can't directly satisfy host interfaces, so
// we bridge the gap: call methods by name via Eval, type-assert the
// concrete return values (strings, sdk.Command, sdk.Result, etc.).
//
// The store fields (documents, tasks, etc.) are per-request holders.
// Exec populates them from the incoming sdk.Context before calling the
// plugin so that the symbol table can point at adapter-owned fields
// rather than package-level globals, giving each adapter isolated,
// request-scoped store access.
type adapter struct {
	i    *interp.Interpreter
	name string
	cmds []sdk.Command

	// mu serialises concurrent Exec calls on the same adapter. Each
	// adapter owns its store fields; concurrent callers would race on
	// them without this lock. Different adapters (different plugins) are
	// fully independent and run concurrently.
	mu sync.Mutex

	// Per-request store holders — populated by Exec before each plugin
	// call. The symbol table points at these fields so Yaegi reads the
	// request-scoped bridges rather than package-level globals.
	documents  sdk.DocumentStore
	tasks      sdk.TaskStore
	links      sdk.LinkStore
	tags       sdk.TagStore
	activities sdk.ActivityStore
	mirror     sdk.MirrorStore
}

func (a *adapter) Name() string            { return a.name }
func (a *adapter) Commands() []sdk.Command { return a.cmds }

func (a *adapter) Exec(ctx sdk.Context, cmd string, args []string) (resp sdk.Response, err error) {
	// Serialise concurrent calls on this adapter — the store fields are
	// shared state on the adapter and must not be overwritten mid-flight.
	a.mu.Lock()
	defer a.mu.Unlock()

	// Populate per-adapter store holders from the incoming request context
	// so the symbol table has the right bridges for this call.
	a.documents = ctx.Documents
	a.tasks = ctx.Tasks
	a.links = ctx.Links
	a.tags = ctx.Tags
	a.activities = ctx.Activities
	a.mirror = ctx.Mirror

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("plugin %s panicked: %v\n%s", a.name, r, debug.Stack())
		}
	}()

	// Build []string literal for args
	var ab strings.Builder
	ab.WriteString("[]string{")
	for i, arg := range args {
		if i > 0 {
			ab.WriteString(", ")
		}
		fmt.Fprintf(&ab, "%q", arg)
	}
	ab.WriteString("}")

	stdinLit := "nil"
	if ctx.Stdin != nil {
		stdinLit = fmt.Sprintf("[]byte(%q)", string(ctx.Stdin))
	}

	expr := fmt.Sprintf(`_r, _e = _p.Exec(sdk.Context{Author: %q, Stdin: %s}, %q, %s)`,
		ctx.Author, stdinLit, cmd, ab.String())
	if _, err := a.i.Eval(expr); err != nil {
		return nil, err
	}

	rv, err := a.i.Eval(`_r`)
	if err != nil {
		return nil, err
	}
	ev, err := a.i.Eval(`_e`)
	if err != nil {
		return nil, err
	}

	var result sdk.Response
	if ri := rv.Interface(); ri != nil {
		if r, ok := ri.(sdk.Response); ok {
			result = r
		} else {
			slog.Warn("plugin returned unexpected response type", "plugin", a.name, "type", fmt.Sprintf("%T", ri))
		}
	}

	var execErr error
	if ei := ev.Interface(); ei != nil {
		if e, ok := ei.(error); ok {
			execErr = e
		} else {
			slog.Warn("plugin returned unexpected error type", "plugin", a.name, "type", fmt.Sprintf("%T", ei))
		}
	}

	return result, execErr
}

// load creates a Yaegi interpreter, evaluates the plugin source, and
// builds an adapter that implements sdk.Plugin via Eval calls.
func load(dir string) (sdk.Plugin, error) {
	src, err := readSource(dir)
	if err != nil {
		return nil, err
	}

	pkg := pkgName(src)
	if pkg == "" {
		return nil, ErrNoGoFiles
	}

	i := interp.New(interp.Options{})

	// Create the adapter before registering symbols so that a.symbols()
	// can point at its store fields. Exec will populate those fields from
	// the incoming sdk.Context before each plugin call.
	a := &adapter{i: i}

	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf("loading stdlib: %w", err)
	}
	if err := i.Use(a.symbols()); err != nil {
		return nil, fmt.Errorf("loading symbols: %w", err)
	}

	// Eval the plugin source (defines the package with New(), types, etc.)
	if _, err := i.Eval(src); err != nil {
		return nil, fmt.Errorf("eval: %w", err)
	}

	// Import sdk in main scope so we can declare typed variables
	if _, err := i.Eval(`import "github.com/jpl-au/llmd/sdk"`); err != nil {
		return nil, fmt.Errorf("sdk import: %w", err)
	}

	// Create plugin instance in interpreter scope
	if _, err := i.Eval(fmt.Sprintf("var _p = %s.New()", pkg)); err != nil {
		return nil, fmt.Errorf("New(): %w", err)
	}

	// Declare result variables for Exec bridge
	if _, err := i.Eval(`var _r sdk.Response`); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	if _, err := i.Eval(`var _e error`); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}

	// Cache Name
	v, err := i.Eval(`_p.Name()`)
	if err != nil {
		return nil, fmt.Errorf("Name(): %w", err)
	}
	name, ok := v.Interface().(string)
	if !ok {
		return nil, ErrNotPlugin
	}
	a.name = name

	// Cache Commands
	v, err = i.Eval(`_p.Commands()`)
	if err != nil {
		return nil, fmt.Errorf("Commands(): %w", err)
	}
	a.cmds, _ = v.Interface().([]sdk.Command)

	return a, nil
}

// readSource concatenates all .go files in dir into a single source string.
// It strips package declarations from all but the first file.
//
// Import statements (including the sdk import) are preserved — Yaegi needs
// them for namespace resolution even when symbols are provided via Use().
func readSource(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	first := true
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return "", err
		}
		src := string(data)

		if first {
			first = false
		} else {
			// Replace (not remove) the package declaration so that all
			// subsequent line numbers stay correct for the //line directive.
			src = stripPackageDecl(src)
		}
		// Inject a //line directive so Yaegi reports errors against the
		// original filename and line number, not the concatenated source.
		fmt.Fprintf(&sb, "//line %s:1\n", e.Name())
		sb.WriteString(src)
		sb.WriteByte('\n')
	}

	if sb.Len() == 0 {
		return "", ErrNoGoFiles
	}
	return sb.String(), nil
}

// stripPackageDecl replaces the "package ..." declaration with a blank
// line. Replacing rather than removing preserves line numbers so that
// //line directives correctly map Yaegi errors back to the original file.
func stripPackageDecl(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			lines[i] = ""
			break
		}
	}
	return strings.Join(lines, "\n")
}

// pkgName extracts the package name from Go source.
func pkgName(src string) string {
	for line := range strings.SplitSeq(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			return strings.TrimSpace(trimmed[len("package "):])
		}
	}
	return ""
}
