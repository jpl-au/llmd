package cli

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

var routeWildcard = regexp.MustCompile(`\{([a-z]+)(\.\.\.)?\}`)

// declaredRoutes returns every command that declares HTTP routes.
func declaredRoutes() map[string][]sdk.Route {
	out := make(map[string][]sdk.Route)
	for _, cmd := range (&CLI{}).Commands() {
		if len(cmd.Routes) > 0 {
			out[cmd.Name] = cmd.Routes
		}
	}
	return out
}

// TestRoutesWellFormed checks every declared route: the method is one
// the server registers, the pattern starts with the command name, each
// wildcard in the pattern is used by the argument template and each
// template reference names a wildcard, and only the last wildcard may
// span several segments.
func TestRoutesWellFormed(t *testing.T) {
	for name, routes := range declaredRoutes() {
		for _, rt := range routes {
			label := rt.Method + " " + rt.Pattern
			switch rt.Method {
			case "GET", "POST", "DELETE":
			default:
				t.Errorf("%s: method %q is not GET, POST or DELETE", label, rt.Method)
			}
			// unlink lives under /link as the DELETE route.
			prefix := "/" + name
			if name == "unlink" {
				prefix = "/link/"
			}
			if !strings.HasPrefix(rt.Pattern, prefix) {
				t.Errorf("%s: pattern does not start with %s", label, prefix)
			}

			wild := make(map[string]bool)
			matches := routeWildcard.FindAllStringSubmatchIndex(rt.Pattern, -1)
			for i, m := range matches {
				wild[rt.Pattern[m[2]:m[3]]] = true
				if m[4] != -1 && i != len(matches)-1 {
					t.Errorf("%s: a {name...} wildcard must be last", label)
				}
			}
			used := make(map[string]bool)
			for _, a := range rt.Args {
				// {?name} is a query parameter, not a wildcard.
				if strings.HasPrefix(a, "{") && !strings.HasPrefix(a, "{?") && strings.HasSuffix(a, "}") {
					w := a[1 : len(a)-1]
					used[w] = true
					if !wild[w] {
						t.Errorf("%s: template refers to {%s}, which the pattern does not capture", label, w)
					}
				}
			}
			for w := range wild {
				if !used[w] {
					t.Errorf("%s: pattern captures {%s}, which the template never uses", label, w)
				}
			}
		}
	}
}

// identityReads are GET routes that read on behalf of the caller and
// so need an author to know whose view to return. They change nothing.
var identityReads = map[string]string{
	"GET /audit/status":  "the inbox belongs to the author",
	"GET /queue":         "pending messages are per consumer",
	"GET /queue/ls":      "pending messages are per consumer",
	"GET /queue/peek":    "pending messages are per consumer",
	"GET /queue/history": "history is filtered for the consumer",
}

// TestRoutesGetNeverMutates dispatches every GET route with an empty
// author. A subcommand that changes the store insists on an author,
// so a GET route that reaches one fails here with sdk.ErrMissingArg.
// Reads listed in identityReads are the only ones allowed to ask for
// an author.
func TestRoutesGetNeverMutates(t *testing.T) {
	for name, routes := range declaredRoutes() {
		for _, rt := range routes {
			if rt.Method != "GET" || identityReads[rt.Method+" "+rt.Pattern] != "" {
				continue
			}
			t.Run(rt.Pattern, func(t *testing.T) {
				c := newCLI(t, "")
				var args []string
				for _, a := range rt.Args {
					if strings.HasPrefix(a, "{") {
						a = "placeholder"
					}
					args = append(args, a)
				}
				err := runErr(t, c, name, nil, args...)
				if errors.Is(err, sdk.ErrMissingArg) && strings.Contains(err.Error(), "author") {
					t.Errorf("GET %s dispatches %s %v, which requires an author: %v", rt.Pattern, name, args, err)
				}
			})
		}
	}
}

// TestRoutesDocumented checks that guide/serve.md lists every declared
// route, written as METHOD /pattern with wildcards as <name>.
func TestRoutesDocumented(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "guide", "serve.md"))
	if err != nil {
		t.Fatalf("reading guide: %v", err)
	}
	guide := string(data)
	for _, routes := range declaredRoutes() {
		for _, rt := range routes {
			shown := rt.Method + " " + routeWildcard.ReplaceAllString(rt.Pattern, "<$1>")
			if !strings.Contains(guide, "`"+shown+"`") {
				t.Errorf("guide/serve.md does not list `%s`", shown)
			}
		}
	}
}
