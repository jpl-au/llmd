package host

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jpl-au/llmd/extension"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// eventRecorder is an extension.EventHandler that keeps every event
// it receives. err, when set, is returned from HandleEvent.
type eventRecorder struct {
	mu     sync.Mutex
	events []extension.Event
	err    error
}

func (r *eventRecorder) HandleEvent(_ extension.Context, e extension.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return r.err
}

// all returns a copy of the recorded events.
func (r *eventRecorder) all() []extension.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]extension.Event(nil), r.events...)
}

// reset forgets the recorded events.
func (r *eventRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

// testExtension is a compiled extension registered by the tests. It
// records the events the host bridges to it and owns one streaming
// command that needs no store, so Run's streaming path can be covered
// without starting a server.
type testExtension struct {
	eventRecorder
	name string
}

func (e *testExtension) Name() string { return e.name }

func (e *testExtension) Commands() []sdk.Command {
	return []sdk.Command{{Name: e.streamCmd(), Desc: "test streaming command", Streams: true}}
}

func (e *testExtension) Exec(_ sdk.Context, cmd string, _ []string) (sdk.Response, error) {
	if cmd != e.streamCmd() {
		return nil, sdk.ErrUnknownCmd
	}
	return sdk.Text("streamed"), nil
}

func (e *testExtension) streamCmd() string { return e.name + "-stream" }

var (
	testExtOnce sync.Once
	testExt     *testExtension
)

// registerTestExtension registers the test extension once per process
// (the registry panics on a duplicate name) under a name derived from
// the first registering test, resets its recorder and returns it.
// Hosts built after this call wire the extension's event handler and
// command table.
func registerTestExtension(t *testing.T) *testExtension {
	t.Helper()
	testExtOnce.Do(func() {
		name := strings.ToLower(strings.NewReplacer("/", "-", "_", "-", " ", "-").Replace(t.Name()))
		testExt = &testExtension{name: "host-test-" + name}
		extension.Register(testExt)
	})
	testExt.reset()
	return testExt
}

// bridgeHost returns a host over an isolated store with an event
// bridge subscribed for the returned recorder.
func bridgeHost(t *testing.T) (sdk.Context, *eventRecorder) {
	t.Helper()
	h := TestSetup(t, TestIsolated)
	store := h.current()
	rec := &eventRecorder{}
	store.Bus().Subscribe(&eventBridge{
		handlers: []extension.EventHandler{rec},
		ctx:      extension.NewContext(store, store.DB(), nil),
	})
	return h.Context(t.Context(), "alice"), rec
}

// TestEventBridge performs one store operation per case through the
// SDK bridges and checks the extension event the bridge delivers for
// it. Each op returns the event it expects, built from the identifiers
// the operation produced, or nil for an operation that has no
// extension counterpart.
func TestEventBridge(t *testing.T) {
	tests := []struct {
		name string
		op   func(t *testing.T, c sdk.Context) extension.Event
	}{
		{
			name: "document write",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "docs/a", "x")
				return extension.DocumentWriteEvent{Path: "docs/a", Version: 1, Author: "alice"}
			},
		},
		{
			name: "document delete",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "docs/a", "x")
				if err := c.Documents.Delete("docs/a", sdk.DeleteOpts{Author: "alice"}); err != nil {
					t.Fatalf("Delete: %v", err)
				}
				return extension.DocumentDeleteEvent{Path: "docs/a"}
			},
		},
		{
			name: "document restore",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "docs/a", "x")
				writeDoc(t, c, "docs/a", "y")
				if err := c.Documents.Delete("docs/a", sdk.DeleteOpts{Author: "alice"}); err != nil {
					t.Fatalf("Delete: %v", err)
				}
				if err := c.Documents.Restore("docs/a", sdk.RestoreOpts{Author: "alice"}); err != nil {
					t.Fatalf("Restore: %v", err)
				}
				return extension.DocumentRestoreEvent{Path: "docs/a", Version: 2}
			},
		},
		{
			name: "document move",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "old", "x")
				if err := c.Documents.Move("old", "new", sdk.MoveOpts{Author: "alice"}); err != nil {
					t.Fatalf("Move: %v", err)
				}
				return extension.DocumentMoveEvent{Path: "new", OldPath: "old", Version: 1}
			},
		},
		{
			name: "tag add",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "docs/a", "x")
				if err := c.Tags.Add("docs/a", "urgent", "alice"); err != nil {
					t.Fatalf("Tags.Add: %v", err)
				}
				return extension.TagEvent{Path: "docs/a", Tag: "urgent", Added: true}
			},
		},
		{
			name: "tag remove",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "docs/a", "x")
				if err := c.Tags.Add("docs/a", "urgent", "alice"); err != nil {
					t.Fatalf("Tags.Add: %v", err)
				}
				if err := c.Tags.Remove("docs/a", "urgent", "alice"); err != nil {
					t.Fatalf("Tags.Remove: %v", err)
				}
				return extension.TagEvent{Path: "docs/a", Tag: "urgent", Added: false}
			},
		},
		{
			name: "link create",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "a", "x")
				writeDoc(t, c, "b", "y")
				if err := c.Links.Add("a", "b", "refs", "alice"); err != nil {
					t.Fatalf("Links.Add: %v", err)
				}
				return extension.LinkEvent{FromPath: "a", ToPath: "b", Tag: "refs", Created: true}
			},
		},
		{
			name: "link remove",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				writeDoc(t, c, "a", "x")
				writeDoc(t, c, "b", "y")
				if err := c.Links.Add("a", "b", "refs", "alice"); err != nil {
					t.Fatalf("Links.Add: %v", err)
				}
				if err := c.Links.Remove("a", "b", "alice"); err != nil {
					t.Fatalf("Links.Remove: %v", err)
				}
				// Remove takes no label, so the event carries none.
				return extension.LinkEvent{FromPath: "a", ToPath: "b", Created: false}
			},
		},
		{
			name: "audit create",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				aud, err := c.Audits.Add(sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
				if err != nil {
					t.Fatalf("Audits.Add: %v", err)
				}
				return extension.AuditEvent{Type: extension.EventAuditCreate, ID: aud.ID, Target: "docs/a", Author: "alice", Status: "pending"}
			},
		},
		{
			name: "audit reply",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				aud, err := c.Audits.Add(sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice"})
				if err != nil {
					t.Fatalf("Audits.Add: %v", err)
				}
				reply, err := c.Audits.Reply(aud.ID, sdk.AuditOpts{Content: "done", Author: "bob", Status: "needs-work"})
				if err != nil {
					t.Fatalf("Audits.Reply: %v", err)
				}
				return extension.AuditEvent{Type: extension.EventAuditReply, ID: reply.ID, Target: "docs/a", Author: "bob", Status: "needs-work", ParentID: aud.ID}
			},
		},
		{
			name: "audit resolve",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				aud, err := c.Audits.Add(sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice"})
				if err != nil {
					t.Fatalf("Audits.Add: %v", err)
				}
				res, err := c.Audits.Resolve(aud.ID, "bob")
				if err != nil {
					t.Fatalf("Audits.Resolve: %v", err)
				}
				return extension.AuditEvent{Type: extension.EventAuditResolve, ID: res.ID, Target: "docs/a", Author: "bob", Status: "approved", ParentID: aud.ID}
			},
		},
		{
			name: "audit delete",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				aud, err := c.Audits.Add(sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice"})
				if err != nil {
					t.Fatalf("Audits.Add: %v", err)
				}
				if err := c.Audits.Delete(aud.ID, "bob"); err != nil {
					t.Fatalf("Audits.Delete: %v", err)
				}
				return extension.AuditEvent{Type: extension.EventAuditDelete, ID: aud.ID, Author: "bob"}
			},
		},
		{
			name: "audit restore",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				aud, err := c.Audits.Add(sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice"})
				if err != nil {
					t.Fatalf("Audits.Add: %v", err)
				}
				if err := c.Audits.Delete(aud.ID, "bob"); err != nil {
					t.Fatalf("Audits.Delete: %v", err)
				}
				if _, err := c.Audits.Restore(aud.ID, "carol"); err != nil {
					t.Fatalf("Audits.Restore: %v", err)
				}
				return extension.AuditEvent{Type: extension.EventAuditRestore, ID: aud.ID, Target: "docs/a", Author: "carol"}
			},
		},
		{
			name: "task create",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				task := addTask(t, c, "Build it", nil, sdk.TaskAddOpts{Author: "alice"})
				return extension.TaskEvent{Type: extension.EventTaskCreate, Key: task.Key, Path: task.Path, Author: "alice", Title: "Build it", Status: "backlog"}
			},
		},
		{
			name: "task move",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				task := addTask(t, c, "Build it", spec, sdk.TaskAddOpts{Author: "alice"})
				moveTask(t, c, task.Key, "in-progress")
				return extension.TaskEvent{Type: extension.EventTaskMove, Key: task.Key, Path: task.Path, Author: "alice", Status: "in-progress"}
			},
		},
		{
			name: "task update",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				task := addTask(t, c, "Build it", nil, sdk.TaskAddOpts{Author: "alice"})
				if err := c.Tasks.Set(task.Key, "bob", sdk.TaskSetOpts{Priority: ptr(2)}); err != nil {
					t.Fatalf("Tasks.Set: %v", err)
				}
				return extension.TaskEvent{Type: extension.EventTaskUpdate, Key: task.Key, Path: task.Path, Author: "bob"}
			},
		},
		{
			name: "task delete",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				task := addTask(t, c, "Build it", nil, sdk.TaskAddOpts{Author: "alice"})
				if _, err := c.Tasks.Delete(task.Key, "bob"); err != nil {
					t.Fatalf("Tasks.Delete: %v", err)
				}
				return extension.TaskEvent{Type: extension.EventTaskDelete, Key: task.Key, Path: task.Path, Author: "bob"}
			},
		},
		{
			name: "task restore",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				task := addTask(t, c, "Build it", nil, sdk.TaskAddOpts{Author: "alice"})
				if _, err := c.Tasks.Delete(task.Key, "bob"); err != nil {
					t.Fatalf("Tasks.Delete: %v", err)
				}
				if _, err := c.Tasks.Restore(task.Key, "carol"); err != nil {
					t.Fatalf("Tasks.Restore: %v", err)
				}
				return extension.TaskEvent{Type: extension.EventTaskRestore, Key: task.Key, Path: task.Path, Author: "carol"}
			},
		},
		{
			name: "queue message has no extension event",
			op: func(t *testing.T, c sdk.Context) extension.Event {
				if _, err := c.Queue.Send(sdk.SendOpts{Type: "direct", Payload: "hi", Author: "alice"}); err != nil {
					t.Fatalf("Queue.Send: %v", err)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := bridgeHost(t)
			want := tt.op(t, c)
			got := rec.all()

			if want == nil {
				if len(got) != 0 {
					t.Errorf("bridge delivered %d events, want none: %+v", len(got), got)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("bridge delivered no events, want %+v", want)
			}
			last := got[len(got)-1]
			// Link keys are generated; check presence, then compare
			// the rest.
			if le, ok := last.(extension.LinkEvent); ok {
				if le.Created && le.ID == "" {
					t.Errorf("LinkEvent.ID is empty, want the link key")
				}
				le.ID = ""
				last = le
			}
			if !reflect.DeepEqual(last, want) {
				t.Errorf("last bridged event = %#v, want %#v", last, want)
			}
			if last.EventType() != want.EventType() {
				t.Errorf("EventType() = %q, want %q", last.EventType(), want.EventType())
			}
		})
	}
}

// TestEventBridgeRegisteredExtension registers an extension that
// handles events and checks that a host built afterwards wires it to
// the store's bus.
func TestEventBridgeRegisteredExtension(t *testing.T) {
	ext := registerTestExtension(t)
	c := TestSetup(t, TestIsolated).Context(t.Context(), "alice")

	writeDoc(t, c, "docs/wired", "x")

	want := extension.DocumentWriteEvent{Path: "docs/wired", Version: 1, Author: "alice"}
	for _, e := range ext.all() {
		if reflect.DeepEqual(e, want) {
			return
		}
	}
	t.Errorf("registered extension received %+v, want %+v among them", ext.all(), want)
}

// TestEventBridgeHandleEvent covers the bridge's own dispatch: an
// unmapped event reaches no handler, every handler sees a mapped one,
// and a handler error stops the fan-out and is returned.
func TestEventBridgeHandleEvent(t *testing.T) {
	written := pkgevents.Event{Type: pkgevents.DocumentWritten, Path: "docs/a", Version: 1, Author: "alice"}
	tests := []struct {
		name      string
		handlers  []*eventRecorder
		event     pkgevents.Event
		wantCalls []int // events seen by each handler
		wantErr   string
	}{
		{"unmapped event", []*eventRecorder{{}}, pkgevents.Event{Type: pkgevents.MessageSent}, []int{0}, ""},
		{"fan-out", []*eventRecorder{{}, {}}, written, []int{1, 1}, ""},
		{"handler error stops fan-out", []*eventRecorder{{err: errors.New("boom")}, {}}, written, []int{1, 0}, "boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &eventBridge{}
			for _, h := range tt.handlers {
				b.handlers = append(b.handlers, h)
			}
			err := b.HandleEvent(t.Context(), tt.event)
			if (err != nil) != (tt.wantErr != "") || (err != nil && err.Error() != tt.wantErr) {
				t.Errorf("HandleEvent(%s) error = %v, want %q", tt.event.Type, err, tt.wantErr)
			}
			for i, h := range tt.handlers {
				if got := len(h.all()); got != tt.wantCalls[i] {
					t.Errorf("handler %d saw %d events, want %d", i, got, tt.wantCalls[i])
				}
			}
		})
	}
}

// TestToExtEvent covers the mapping's handling of metadata: absent
// maps, values of the wrong type, and the task move destination
// overriding the status.
func TestToExtEvent(t *testing.T) {
	tests := []struct {
		name string
		in   pkgevents.Event
		want extension.Event
	}{
		{"unknown type", pkgevents.Event{Type: "agent.spawned"}, nil},
		{"move without metadata", pkgevents.Event{Type: pkgevents.DocumentMoved, Path: "new", Version: 3}, extension.DocumentMoveEvent{Path: "new", Version: 3}},
		{"move with wrong metadata type", pkgevents.Event{Type: pkgevents.DocumentMoved, Path: "new", Metadata: map[string]any{"old_path": 7}}, extension.DocumentMoveEvent{Path: "new"}},
		{"tag without metadata", pkgevents.Event{Type: pkgevents.TagRemoved, Path: "doc"}, extension.TagEvent{Path: "doc"}},
		{"link without metadata", pkgevents.Event{Type: pkgevents.LinkRemoved, Path: "a", Key: "k"}, extension.LinkEvent{ID: "k", FromPath: "a"}},
		{"link with partial metadata", pkgevents.Event{Type: pkgevents.LinkCreated, Path: "a", Metadata: map[string]any{"to": "b", "label": 1}}, extension.LinkEvent{FromPath: "a", ToPath: "b", Created: true}},
		{"audit without metadata", pkgevents.Event{Type: pkgevents.AuditDeleted, Key: "id", Author: "bob"}, extension.AuditEvent{Type: extension.EventAuditDelete, ID: "id", Author: "bob"}},
		{"audit with wrong metadata types", pkgevents.Event{Type: pkgevents.AuditReplied, Key: "id", Metadata: map[string]any{"status": 1, "parent_id": true}}, extension.AuditEvent{Type: extension.EventAuditReply, ID: "id"}},
		{"task status from metadata", pkgevents.Event{Type: pkgevents.TaskCreated, Key: "k", Metadata: map[string]any{"title": "T", "status": "backlog"}}, extension.TaskEvent{Type: extension.EventTaskCreate, Key: "k", Title: "T", Status: "backlog"}},
		{"task move destination wins", pkgevents.Event{Type: pkgevents.TaskMoved, Key: "k", Metadata: map[string]any{"status": "backlog", "to": "done"}}, extension.TaskEvent{Type: extension.EventTaskMove, Key: "k", Status: "done"}},
		{"task with wrong metadata types", pkgevents.Event{Type: pkgevents.TaskUpdated, Key: "k", Metadata: map[string]any{"title": 1, "status": 2, "to": 3}}, extension.TaskEvent{Type: extension.EventTaskUpdate, Key: "k"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toExtEvent(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toExtEvent(%s) = %#v, want %#v", tt.in.Type, got, tt.want)
			}
		})
	}
}
