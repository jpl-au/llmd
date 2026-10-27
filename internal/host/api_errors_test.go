package host

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// notFoundCase drives one SDK store method against an identifier that
// does not exist.
type notFoundCase struct {
	store  string // SDK interface name, e.g. "DocumentStore"
	method string
	call   func(c sdk.Context) error
}

// notFoundCases lists every SDK store method that addresses a single
// resource. Each must report sdk.ErrNotFound for a missing identifier,
// whatever the internal package returned, so that CLI, MCP and HTTP
// callers can rely on errors.Is(err, sdk.ErrNotFound) uniformly.
var notFoundCases = []notFoundCase{
	// DocumentStore
	{"DocumentStore", "Read", func(c sdk.Context) error {
		_, err := c.Documents.Read("missing", sdk.ReadOpts{})
		return err
	}},
	{"DocumentStore", "Delete", func(c sdk.Context) error {
		return c.Documents.Delete("missing", sdk.DeleteOpts{Author: "alice"})
	}},
	{"DocumentStore", "Restore", func(c sdk.Context) error {
		return c.Documents.Restore("missing", sdk.RestoreOpts{Author: "alice"})
	}},
	{"DocumentStore", "Move", func(c sdk.Context) error {
		return c.Documents.Move("missing", "dest", sdk.MoveOpts{Author: "alice"})
	}},
	{"DocumentStore", "Edit", func(c sdk.Context) error {
		return c.Documents.Edit("missing", "old", "new", sdk.EditOpts{Author: "alice"})
	}},
	{"DocumentStore", "History", func(c sdk.Context) error {
		_, err := c.Documents.History("missing", 0)
		return err
	}},
	{"DocumentStore", "Diff", func(c sdk.Context) error {
		_, _, _, err := c.Documents.Diff("missing", "missing", 0)
		return err
	}},
	{"DocumentStore", "Revert", func(c sdk.Context) error {
		return c.Documents.Revert("missing", 1, sdk.RevertOpts{Author: "alice"})
	}},
	{"DocumentStore", "Preview", func(c sdk.Context) error {
		_, err := c.Documents.Preview("missing", 5)
		return err
	}},
	{"DocumentStore", "Export", func(c sdk.Context) error {
		_, err := c.Documents.Export("missing", "out", sdk.ExportOpts{})
		return err
	}},

	// TaskStore
	{"TaskStore", "Read", func(c sdk.Context) error {
		_, err := c.Tasks.Read("missing")
		return err
	}},
	{"TaskStore", "Move", func(c sdk.Context) error {
		return c.Tasks.Move("missing", "done", "alice")
	}},
	{"TaskStore", "Set", func(c sdk.Context) error {
		return c.Tasks.Set("missing", "alice", sdk.TaskSetOpts{Title: new("x")})
	}},
	{"TaskStore", "Delete", func(c sdk.Context) error {
		_, err := c.Tasks.Delete("missing", "alice")
		return err
	}},
	{"TaskStore", "Restore", func(c sdk.Context) error {
		_, err := c.Tasks.Restore("missing", "alice")
		return err
	}},
	{"TaskStore", "RemoveColumn", func(c sdk.Context) error {
		return c.Tasks.RemoveColumn("missing", "alice")
	}},
	{"TaskStore", "MoveColumn", func(c sdk.Context) error {
		return c.Tasks.MoveColumn("missing", "done", "alice")
	}},
	{"TaskStore", "Start", func(c sdk.Context) error {
		_, err := c.Tasks.Start("missing", "alice", sdk.StartOpts{})
		return err
	}},
	{"TaskStore", "StartBranch", func(c sdk.Context) error {
		_, err := c.Tasks.StartBranch("missing", "alice", sdk.StartBranchOpts{})
		return err
	}},
	{"TaskStore", "Finish", func(c sdk.Context) error {
		_, err := c.Tasks.Finish("missing", "alice", sdk.FinishOpts{})
		return err
	}},
	{"TaskStore", "ByBranch", func(c sdk.Context) error {
		_, err := c.Tasks.ByBranch("missing")
		return err
	}},
	{"TaskStore", "Link", func(c sdk.Context) error {
		return c.Tasks.Link("missing", "docs/x", "alice")
	}},
	{"TaskStore", "Links", func(c sdk.Context) error {
		_, err := c.Tasks.Links("missing", "both")
		return err
	}},
	{"TaskStore", "Dep", func(c sdk.Context) error {
		_, err := c.Tasks.Dep("missing")
		return err
	}},
	{"TaskStore", "Dependents", func(c sdk.Context) error {
		_, err := c.Tasks.Dependents("missing")
		return err
	}},
	{"TaskStore", "Chain", func(c sdk.Context) error {
		_, err := c.Tasks.Chain("missing")
		return err
	}},
	{"TaskStore", "Ready", func(c sdk.Context) error {
		_, err := c.Tasks.Ready("missing")
		return err
	}},
	{"TaskStore", "Log", func(c sdk.Context) error {
		_, err := c.Tasks.Log("missing", 0)
		return err
	}},

	// TagStore
	{"TagStore", "Add", func(c sdk.Context) error {
		return c.Tags.Add("missing", "important", "alice")
	}},
	{"TagStore", "Remove", func(c sdk.Context) error {
		return c.Tags.Remove("missing", "important", "alice")
	}},

	// LinkStore
	{"LinkStore", "Add", func(c sdk.Context) error {
		return c.Links.Add("missing", "also-missing", "", "alice")
	}},
	{"LinkStore", "Remove", func(c sdk.Context) error {
		return c.Links.Remove("missing", "also-missing", "alice")
	}},

	// AuditStore
	{"AuditStore", "Reply", func(c sdk.Context) error {
		_, err := c.Audits.Reply("missing", sdk.AuditOpts{Author: "alice", Content: "x"})
		return err
	}},
	{"AuditStore", "Read", func(c sdk.Context) error {
		_, err := c.Audits.Read("missing")
		return err
	}},
	{"AuditStore", "Thread", func(c sdk.Context) error {
		_, err := c.Audits.Thread("missing")
		return err
	}},
	{"AuditStore", "Resolve", func(c sdk.Context) error {
		_, err := c.Audits.Resolve("missing", "alice")
		return err
	}},
	{"AuditStore", "Delete", func(c sdk.Context) error {
		return c.Audits.Delete("missing", "alice")
	}},
	{"AuditStore", "Restore", func(c sdk.Context) error {
		_, err := c.Audits.Restore("missing", "alice")
		return err
	}},

	// QueueStore
	{"QueueStore", "Peek", func(c sdk.Context) error {
		_, err := c.Queue.Peek("nobody")
		return err
	}},
	{"QueueStore", "Ack", func(c sdk.Context) error {
		return c.Queue.Ack("missing", "nobody")
	}},

	// AgentStore
	{"AgentStore", "Remove", func(c sdk.Context) error {
		return c.Agents.Remove("missing", "alice")
	}},
	{"AgentStore", "Agent", func(c sdk.Context) error {
		_, err := c.Agents.Agent("missing")
		return err
	}},
	{"AgentStore", "Spawn", func(c sdk.Context) error {
		_, err := c.Agents.Spawn("missing", "missing", "alice", sdk.SpawnOpts{})
		return err
	}},
	{"AgentStore", "Run", func(c sdk.Context) error {
		_, err := c.Agents.Run("missing")
		return err
	}},
	{"AgentStore", "Complete", func(c sdk.Context) error {
		return c.Agents.Complete("missing", 0)
	}},
	{"AgentStore", "Stop", func(c sdk.Context) error {
		return c.Agents.Stop("missing", "alice")
	}},
	{"AgentStore", "Prompt", func(c sdk.Context) error {
		_, _, err := c.Agents.Prompt("missing", "developer")
		return err
	}},

	// RuleStore
	{"RuleStore", "Unset", func(c sdk.Context) error {
		return c.Rules.Unset("missing")
	}},
}

// notFoundExempt lists, per interface, the methods that do not address
// a single existing resource and therefore have no not-found contract.
// Every method of every store interface must appear either in
// notFoundCases or here; TestNotFound fails otherwise, so a new SDK
// method has to declare its behaviour.
var notFoundExempt = map[string]map[string]string{
	"DocumentStore": {
		"Write":  "creates the document",
		"List":   "returns an empty list",
		"Exists": "reports false",
		"Glob":   "returns no matches",
		"Grep":   "returns no hits",
		"Vacuum": "store-wide",
		"Import": "reads the filesystem",
	},
	"TaskStore": {
		"Next":       "picks from a column; an unknown column is sdk.ErrInvalidArg, an empty one sdk.ErrNoneReady",
		"Add":        "creates the task",
		"List":       "returns an empty list",
		"Columns":    "store-wide",
		"AddColumn":  "creates the column",
		"CheckSpecs": "takes tasks, not identifiers",
	},
	"TagStore": {
		"List": "returns an empty list",
		"All":  "store-wide",
		"Find": "returns no matches",
	},
	"LinkStore": {
		"List": "returns an empty list",
	},
	"AuditStore": {
		"Add":    "creates the audit",
		"List":   "returns an empty list",
		"Status": "returns an empty inbox",
	},
	"QueueStore": {
		"Send":    "creates the message",
		"Pending": "returns an empty list",
		"History": "returns an empty list",
	},
	"AgentStore": {
		"Register":    "creates the agent",
		"Agents":      "returns an empty list",
		"Runs":        "returns an empty list",
		"WritePrompt": "creates the prompt file",
	},
	"ActivityStore": {
		"Recent": "store-wide",
	},
	"MirrorStore": {
		"Directory": "store-wide",
		"Pull":      "prefix export to the filesystem; empty prefix is not an error",
		"Push":      "reads the filesystem",
	},
	"RuleStore": {
		"Show": "store-wide",
		"Set":  "creates the rule",
	},
	"HookStore": {
		"Handle": "acts on a payload; a missing task is logged and skipped",
	},
}

// notFoundStores maps interface names to their reflect types so the
// exemption check can enumerate every method.
var notFoundStores = map[string]reflect.Type{
	"DocumentStore": reflect.TypeFor[sdk.DocumentStore](),
	"TaskStore":     reflect.TypeFor[sdk.TaskStore](),
	"TagStore":      reflect.TypeFor[sdk.TagStore](),
	"LinkStore":     reflect.TypeFor[sdk.LinkStore](),
	"AuditStore":    reflect.TypeFor[sdk.AuditStore](),
	"QueueStore":    reflect.TypeFor[sdk.QueueStore](),
	"AgentStore":    reflect.TypeFor[sdk.AgentStore](),
	"ActivityStore": reflect.TypeFor[sdk.ActivityStore](),
	"MirrorStore":   reflect.TypeFor[sdk.MirrorStore](),
	"RuleStore":     reflect.TypeFor[sdk.RuleStore](),
	"HookStore":     reflect.TypeFor[sdk.HookStore](),
}

// TestNotFound is the guard for the SDK error contract: every store
// method that addresses a resource reports sdk.ErrNotFound for a
// missing identifier, and every store method is accounted for either
// as a case or as an explicit exemption.
func TestNotFound(t *testing.T) {
	covered := make(map[string]map[string]bool)
	for _, tt := range notFoundCases {
		if covered[tt.store] == nil {
			covered[tt.store] = make(map[string]bool)
		}
		covered[tt.store][tt.method] = true
		t.Run(tt.store+"/"+tt.method, func(t *testing.T) {
			c := testHost(t)
			if err := tt.call(c); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("%s.%s(missing) error = %v, want sdk.ErrNotFound", tt.store, tt.method, err)
			}
		})
	}

	t.Run("every method is accounted for", func(t *testing.T) {
		for name, typ := range notFoundStores {
			for method := range typ.Methods() {
				m := method.Name
				if covered[name][m] || notFoundExempt[name][m] != "" {
					continue
				}
				t.Errorf("%s.%s has neither a not-found case nor an exemption", name, m)
			}
			for m := range notFoundExempt[name] {
				if _, ok := typ.MethodByName(m); !ok {
					t.Errorf("%s.%s is exempted but no longer exists", name, m)
				}
				if covered[name][m] {
					t.Errorf("%s.%s is both a case and an exemption", name, m)
				}
			}
		}
	})
}
