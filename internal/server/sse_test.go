package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/pkg/events"
)

// waitFor bounds every wait on a channel in these tests. Nothing here
// should take anywhere near this long; it only stops a broken stream
// from hanging the suite.
const waitFor = 5 * time.Second

// sseConn is a connected SSE client. A goroutine reads the stream
// line by line onto lines until the request is cancelled or the
// server closes the stream, then closes done.
type sseConn struct {
	resp   *http.Response
	lines  chan string
	done   chan struct{}
	cancel context.CancelFunc
	once   sync.Once
}

// connectSSE opens GET url with a cancellable request and returns once
// the server has sent its headers, by which point ServeHTTP has
// registered the client in the hub. The connection is closed on
// cleanup, before the test server it belongs to.
func connectSSE(t *testing.T, client *http.Client, url string) *sseConn {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("GET %s: %v", url, err)
	}
	c := &sseConn{
		resp:   resp,
		lines:  make(chan string, 256),
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go func() {
		defer close(c.done)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			select {
			case c.lines <- sc.Text():
			case <-ctx.Done():
				return
			}
		}
	}()
	t.Cleanup(func() { c.close(t) })
	return c
}

// close cancels the request and waits for the reader goroutine to
// stop. It is safe to call more than once.
func (c *sseConn) close(t *testing.T) {
	t.Helper()
	c.once.Do(func() {
		c.cancel()
		select {
		case <-c.done:
		case <-time.After(waitFor):
			t.Error("sse reader did not stop after cancel")
		}
		c.resp.Body.Close()
	})
}

// next reads one event from the stream: the event line, the data line
// and the blank line that ends it. It fails the test if no event
// arrives within waitFor.
func (c *sseConn) next(t *testing.T) (string, events.Event) {
	t.Helper()
	deadline := time.After(waitFor)
	var name, data string
	for {
		select {
		case line := <-c.lines:
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			case line == "" && name != "":
				var e events.Event
				if err := json.Unmarshal([]byte(data), &e); err != nil {
					t.Fatalf("sse data %q is not an event: %v", data, err)
				}
				return name, e
			}
		case <-deadline:
			t.Fatalf("no sse event within %v (event=%q data=%q)", waitFor, name, data)
		}
	}
}

// count returns how many clients the hub currently has.
func (h *sseHub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// TestSSEStream connects a client to GET /events on a server built by
// New, emits events through the callback the server subscribed on the
// runtime, and checks the event and data lines that arrive. Delivery
// is in order, so a case that sends a filtered-out event first and
// expects the later one proves the filter dropped it. The response
// headers are checked for every case.
func TestSSEStream(t *testing.T) {
	written := events.Event{Type: events.DocumentWritten, Path: "docs/a", Key: "k1", Version: 2, Author: "claude", Timestamp: 1700000000000}
	created := events.Event{Type: events.AuditCreated, Path: "docs/a", Key: "k2", Author: "gemini", Timestamp: 1700000000001, Metadata: map[string]any{"status": "pending"}}
	moved := events.Event{Type: events.TaskMoved, Key: "k3", Author: "claude", Timestamp: 1700000000002, Metadata: map[string]any{"column": "review"}}
	bad := events.Event{Type: events.TagAdded, Key: "k4", Timestamp: 1700000000003, Metadata: map[string]any{"fn": func() {}}}

	tests := []struct {
		name  string
		types string // the ?type= filter
		send  []events.Event
		want  []events.Event
	}{
		{name: "all events", send: []events.Event{written, created, moved}, want: []events.Event{written, created, moved}},
		{name: "one type", types: "audit.created", send: []events.Event{written, created}, want: []events.Event{created}},
		{name: "several types", types: "audit.created,task.moved", send: []events.Event{written, created, moved}, want: []events.Event{created, moved}},
		{name: "types with spaces", types: "audit.created, task.moved", send: []events.Event{written, moved}, want: []events.Event{moved}},
		{name: "unmarshallable event is skipped", send: []events.Event{bad, written}, want: []events.Event{written}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := stubRuntime()
			ts := newTestServer(t, rt)
			if n := rt.subscribers(); n != 1 {
				t.Fatalf("New subscribed %d callbacks, want 1", n)
			}

			u := ts.URL + "/events"
			if tt.types != "" {
				u += "?" + url.Values{"type": {tt.types}}.Encode()
			}
			conn := connectSSE(t, ts.Client(), u)

			if got := conn.resp.Header.Get("Content-Type"); got != "text/event-stream" {
				t.Errorf("GET /events Content-Type = %q, want %q", got, "text/event-stream")
			}
			if got := conn.resp.Header.Get("Cache-Control"); got != "no-cache" {
				t.Errorf("GET /events Cache-Control = %q, want %q", got, "no-cache")
			}

			for _, e := range tt.send {
				rt.emit(e)
			}
			for _, want := range tt.want {
				name, got := conn.next(t)
				if name != want.Type {
					t.Errorf("GET /events?type=%q event line = %q, want %q", tt.types, name, want.Type)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("GET /events?type=%q data = %+v, want %+v", tt.types, got, want)
				}
			}
		})
	}
}

// TestSSETwoClients is a fan-out scenario: two clients with different
// filters, one broadcast each, delivered to exactly the clients whose
// filter matches.
func TestSSETwoClients(t *testing.T) {
	rt := stubRuntime()
	ts := newTestServer(t, rt)

	all := connectSSE(t, ts.Client(), ts.URL+"/events")
	tasks := connectSSE(t, ts.Client(), ts.URL+"/events?type=task.moved")

	written := events.Event{Type: events.DocumentWritten, Path: "docs/a", Timestamp: 1}
	moved := events.Event{Type: events.TaskMoved, Key: "k1", Timestamp: 2}
	rt.emit(written)
	rt.emit(moved)

	for i, want := range []events.Event{written, moved} {
		if name, got := all.next(t); name != want.Type || !reflect.DeepEqual(got, want) {
			t.Errorf("all client event %d = %q %+v, want %q %+v", i, name, got, want.Type, want)
		}
	}
	// The filtered client sees only the task event; it arriving first
	// proves the document event was dropped for it.
	if name, got := tasks.next(t); name != moved.Type || !reflect.DeepEqual(got, moved) {
		t.Errorf("filtered client event = %q %+v, want %q %+v", name, got, moved.Type, moved)
	}
}

// TestSSEDisconnect is a lifecycle scenario: a client that cancels its
// request is removed from the hub once its handler returns, and a
// later broadcast with no clients is a no-op.
func TestSSEDisconnect(t *testing.T) {
	h := newSSEHub()
	served := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		close(served)
	}))
	t.Cleanup(ts.Close)

	conn := connectSSE(t, ts.Client(), ts.URL+"/events")
	if n := h.count(); n != 1 {
		t.Fatalf("after connect hub has %d clients, want 1", n)
	}

	conn.close(t)
	select {
	case <-served:
	case <-time.After(waitFor):
		t.Fatal("ServeHTTP did not return after the client disconnected")
	}
	if n := h.count(); n != 0 {
		t.Errorf("after disconnect hub has %d clients, want 0", n)
	}
	h.Broadcast(events.Event{Type: events.DocumentWritten})
}

// TestSSEHubBroadcast covers the hub's delivery rules on a client
// added directly: no filter takes everything, a filter takes only its
// types, and a client whose buffer is full has the event dropped
// without Broadcast blocking.
func TestSSEHubBroadcast(t *testing.T) {
	tests := []struct {
		name       string
		types      map[string]bool
		buffer     int
		prefill    int
		event      events.Event
		wantQueued int
	}{
		{name: "no filter", buffer: 4, event: events.Event{Type: events.DocumentWritten}, wantQueued: 1},
		{name: "filter match", types: map[string]bool{events.AuditCreated: true}, buffer: 4, event: events.Event{Type: events.AuditCreated}, wantQueued: 1},
		{name: "filter mismatch", types: map[string]bool{events.AuditCreated: true}, buffer: 4, event: events.Event{Type: events.DocumentWritten}, wantQueued: 0},
		{name: "full buffer drops", buffer: 1, prefill: 1, event: events.Event{Type: events.DocumentWritten}, wantQueued: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSSEHub()
			c := &sseClient{events: make(chan events.Event, tt.buffer), types: tt.types, done: make(chan struct{})}
			h.clients[c] = struct{}{}
			for range tt.prefill {
				c.events <- events.Event{Type: "prefill"}
			}

			returned := make(chan struct{})
			go func() {
				defer close(returned)
				h.Broadcast(tt.event)
			}()
			select {
			case <-returned:
			case <-time.After(waitFor):
				t.Fatalf("Broadcast(%v) did not return", tt.event)
			}

			if got := len(c.events); got != tt.wantQueued {
				t.Errorf("Broadcast(%v) with filter %v queued %d events, want %d", tt.event, tt.types, got, tt.wantQueued)
			}
		})
	}
}

// noFlush hides the Flusher a ResponseRecorder implements.
type noFlush struct{ http.ResponseWriter }

// TestSSEServeHTTPNoFlusher checks that a writer that cannot stream is
// refused with a 500 rather than registered.
func TestSSEServeHTTPNoFlusher(t *testing.T) {
	h := newSSEHub()
	rec := httptest.NewRecorder()
	h.ServeHTTP(noFlush{rec}, httptest.NewRequest(http.MethodGet, "/events", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ServeHTTP(no flusher) status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "streaming not supported") {
		t.Errorf("ServeHTTP(no flusher) body = %q, want to contain %q", rec.Body.String(), "streaming not supported")
	}
	if n := h.count(); n != 0 {
		t.Errorf("ServeHTTP(no flusher) left %d clients, want 0", n)
	}
}
