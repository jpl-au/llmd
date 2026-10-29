package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
)

// TestJSONEverywhere holds --json to one meaning for every command:
// the output is JSON, and its keys are snake_case. Write commands once
// printed plain text under --json, and task, audit, history and queue
// keys were PascalCase while ls's were snake_case.
func TestJSONEverywhere(t *testing.T) {
	dir := t.TempDir()
	host.TestHome(t)
	t.Chdir(dir)
	origLog := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origLog) })

	quiet := func(args []string, stdin string) (string, int) {
		var code int
		var out string
		_ = capture(t, &os.Stderr, func() {
			out = capture(t, &os.Stdout, func() {
				if stdin != "" {
					feedStdin(t, stdin, func() { code = run(args) })
				} else {
					code = run(args)
				}
			})
		})
		return out, code
	}
	if _, code := quiet([]string{"init"}, ""); code != 0 {
		t.Fatal("init failed")
	}

	var task string
	steps := []struct {
		args  []string
		stdin string
	}{
		{args: []string{"write", "docs/a"}, stdin: "# A\n\nalpha text"},
		{args: []string{"write", "docs/b"}, stdin: "# B\n\nbeta text"},
		{args: []string{"write", "docs/a"}, stdin: "# A\n\nalpha text, again"},
		{args: []string{"cat", "docs/a"}},
		{args: []string{"ls", "-l"}},
		{args: []string{"history", "docs/a"}},
		{args: []string{"diff", "docs/a"}},
		{args: []string{"edit", "docs/a", "again", "twice"}},
		{args: []string{"sed", "-i", "s/twice/thrice/", "docs/a"}},
		{args: []string{"tag", "docs/a", "red"}},
		{args: []string{"tag", "-f", "red"}},
		{args: []string{"link", "docs/a", "docs/b"}},
		{args: []string{"link", "docs/a"}},
		{args: []string{"grep", "alpha"}},
		{args: []string{"find", "beta"}},
		{args: []string{"glob", "docs/*"}},
		{args: []string{"mv", "docs/b", "docs/c"}},
		{args: []string{"rm", "docs/c"}},
		{args: []string{"restore", "docs/c"}},
		{args: []string{"revert", "docs/a", "1"}},
		{args: []string{"task", "add", "Fix it"}, stdin: "# Fix it\n\nThe spec."},
		{args: []string{"task", "list"}},
		{args: []string{"task", "show", "{task}"}},
		{args: []string{"task", "move", "{task}", "in-progress"}},
		{args: []string{"audit", "add", "docs/a", "looks good"}},
		{args: []string{"audit", "list"}},
		{args: []string{"queue", "send", "hello"}},
		{args: []string{"queue", "ls"}},
		{args: []string{"status"}},
		{args: []string{"review"}},
		{args: []string{"guide", "cat"}},
		{args: []string{"version"}},
		{args: []string{"vacuum", "--dry-run"}},
	}
	upper := regexp.MustCompile(`^[A-Z]`)
	for _, st := range steps {
		args := make([]string, 0, len(st.args)+3)
		for _, a := range st.args {
			args = append(args, strings.ReplaceAll(a, "{task}", task))
		}
		name := strings.Join(args, " ")
		args = append([]string{"--json", "--author", "alice"}, args...)
		out, code := quiet(args, st.stdin)
		if code != 0 {
			t.Errorf("llmd %s exited %d", name, code)
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Errorf("llmd --json %s printed %q, which is not JSON: %v", name, out, err)
			continue
		}
		switch v.(type) {
		case map[string]any, []any:
		default:
			t.Errorf("llmd --json %s printed %s; the top level is an object or an array", name, strings.TrimSpace(out))
		}
		for _, k := range jsonKeys(v) {
			if upper.MatchString(k) {
				t.Errorf("llmd --json %s has the key %q; keys are snake_case", name, k)
			}
		}
		if task == "" && st.args[0] == "task" && st.args[1] == "add" {
			if m, ok := v.(map[string]any); ok {
				task, _ = m["key"].(string)
			}
			if task == "" {
				t.Fatalf("task add --json gave no key: %s", out)
			}
		}
	}
}

// jsonKeys lists every object key in a decoded JSON value.
func jsonKeys(v any) []string {
	var keys []string
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			keys = append(keys, k)
			keys = append(keys, jsonKeys(e)...)
		}
	case []any:
		for _, e := range x {
			keys = append(keys, jsonKeys(e)...)
		}
	}
	return keys
}
