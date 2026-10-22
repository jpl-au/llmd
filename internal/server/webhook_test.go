package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/pkg/events"
)

// TestNewWebhookHub covers the endpoint table built from the
// configuration: no endpoints means no hub, names are sorted, and the
// per-endpoint retries and timeout fall back to the defaults when
// unset or invalid.
func TestNewWebhookHub(t *testing.T) {
	tests := []struct {
		name         string
		cfg          map[string]config.WebhookConfig
		wantNil      bool
		wantNames    []string
		wantAttempts []int
		wantTimeouts []time.Duration
	}{
		{name: "nil config", cfg: nil, wantNil: true},
		{name: "empty config", cfg: map[string]config.WebhookConfig{}, wantNil: true},
		{
			name:         "defaults",
			cfg:          map[string]config.WebhookConfig{"a": {URL: "http://a"}},
			wantNames:    []string{"a"},
			wantAttempts: []int{webhookRetries + 1},
			wantTimeouts: []time.Duration{webhookTimeout},
		},
		{
			name: "sorted with overrides",
			cfg: map[string]config.WebhookConfig{
				"b": {URL: "http://b", Key: "k", Retries: 5, Timeout: "2s"},
				"a": {URL: "http://a", Timeout: "nonsense"},
			},
			wantNames:    []string{"a", "b"},
			wantAttempts: []int{webhookRetries + 1, 6},
			wantTimeouts: []time.Duration{webhookTimeout, 2 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newWebhookHub(tt.cfg)
			if (h == nil) != tt.wantNil {
				t.Fatalf("newWebhookHub(%v) = %v, want nil %v", tt.cfg, h, tt.wantNil)
			}
			if h == nil {
				return
			}
			defer h.Close()
			var names []string
			var attempts []int
			var timeouts []time.Duration
			for _, ep := range h.endpoints {
				names = append(names, ep.name)
				attempts = append(attempts, ep.attempts)
				timeouts = append(timeouts, ep.timeout)
			}
			if !reflect.DeepEqual(names, tt.wantNames) {
				t.Errorf("endpoint names = %v, want %v", names, tt.wantNames)
			}
			if !reflect.DeepEqual(attempts, tt.wantAttempts) {
				t.Errorf("endpoint attempts = %v, want %v", attempts, tt.wantAttempts)
			}
			if !reflect.DeepEqual(timeouts, tt.wantTimeouts) {
				t.Errorf("endpoint timeouts = %v, want %v", timeouts, tt.wantTimeouts)
			}
		})
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		n    int
		want time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second},
		{40, 30 * time.Second},
		{70, 30 * time.Second},
	}
	for _, tt := range tests {
		if got := exponentialBackoff(tt.n); got != tt.want {
			t.Errorf("exponentialBackoff(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

// webhookRecorder captures what a webhook endpoint received. Each
// request is counted on arrived so tests can wait without sleeping.
type webhookRecorder struct {
	mu          sync.Mutex
	auth        string
	contentType string
	received    []events.Event
	arrived     chan struct{}
}

func newWebhookRecorder() *webhookRecorder {
	return &webhookRecorder{arrived: make(chan struct{}, 64)}
}

// handler returns an endpoint that records the request and responds
// with status.
func (r *webhookRecorder) handler(t *testing.T, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()
		defer func() { r.arrived <- struct{}{} }()

		r.auth = req.Header.Get("Authorization")
		r.contentType = req.Header.Get("Content-Type")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("reading body: %v", err)
			return
		}
		var e events.Event
		if err := json.Unmarshal(body, &e); err != nil {
			t.Errorf("unmarshalling event: %v", err)
			return
		}
		r.received = append(r.received, e)
		w.WriteHeader(status)
	})
}

// wait blocks until n requests have arrived or the deadline passes.
func (r *webhookRecorder) wait(t *testing.T, n int) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for range n {
		select {
		case <-r.arrived:
		case <-deadline:
			t.Fatalf("timed out waiting for %d webhook requests", n)
		}
	}
}

// TestWebhookBroadcast sends one event through a real HTTP endpoint
// and checks the headers, the payload, and the retry decision for each
// response: success and 4xx are one attempt, 5xx is retried until the
// attempts are used up.
func TestWebhookBroadcast(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		status       int
		retries      int
		event        events.Event
		wantAuth     string
		wantRequests int
	}{
		{
			name:   "with key",
			key:    "secret",
			status: http.StatusOK,
			event: events.Event{
				Type:      events.AuditCreated,
				Path:      "docs/spec",
				Key:       "abc123",
				Author:    "gemini",
				Timestamp: time.Now().UnixMilli(),
			},
			wantAuth:     "Bearer secret",
			wantRequests: 1,
		},
		{
			name:         "no key",
			status:       http.StatusAccepted,
			event:        events.Event{Type: events.TaskCreated, Timestamp: time.Now().UnixMilli()},
			wantRequests: 1,
		},
		{
			name:         "rejected is not retried",
			key:          "x",
			status:       http.StatusBadRequest,
			event:        events.Event{Type: events.TaskMoved, Timestamp: time.Now().UnixMilli()},
			wantAuth:     "Bearer x",
			wantRequests: 1,
		},
		{
			name:         "server error is retried",
			key:          "x",
			status:       http.StatusInternalServerError,
			event:        events.Event{Type: events.AuditReplied, Timestamp: time.Now().UnixMilli()},
			wantAuth:     "Bearer x",
			wantRequests: webhookRetries + 1,
		},
		{
			name:         "configured retries",
			status:       http.StatusBadGateway,
			retries:      4,
			event:        events.Event{Type: events.DocumentWritten, Timestamp: time.Now().UnixMilli()},
			wantRequests: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := newWebhookRecorder()
			ts := httptest.NewServer(rec.handler(t, tt.status))
			defer ts.Close()

			h := newWebhookHub(map[string]config.WebhookConfig{
				"test": {URL: ts.URL, Key: tt.key, Retries: tt.retries},
			})
			defer h.Close()
			h.backoff = func(int) time.Duration { return 0 }

			h.Broadcast(tt.event)
			rec.wait(t, tt.wantRequests)
			h.Close() // no more attempts can be in flight

			rec.mu.Lock()
			defer rec.mu.Unlock()
			if len(rec.received) != tt.wantRequests {
				t.Fatalf("endpoint got %d requests, want %d", len(rec.received), tt.wantRequests)
			}
			for i, got := range rec.received {
				if !reflect.DeepEqual(got, tt.event) {
					t.Errorf("request %d delivered %+v, want %+v", i, got, tt.event)
				}
			}
			if rec.auth != tt.wantAuth {
				t.Errorf("Authorization = %q, want %q", rec.auth, tt.wantAuth)
			}
			if rec.contentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", rec.contentType, "application/json")
			}
		})
	}
}

// fakeSender records attempts and returns scripted results, so retry
// behaviour can be checked without a network.
type fakeSender struct {
	mu       sync.Mutex
	results  []func() (int, error)
	attempts []string // endpoint names in attempt order
	done     chan struct{}
}

func (f *fakeSender) send(ep webhookEndpoint, _ []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts = append(f.attempts, ep.name)
	i := len(f.attempts) - 1
	status, err := 0, errors.New("unexpected attempt")
	if i < len(f.results) {
		status, err = f.results[i]()
	}
	if f.done != nil {
		f.done <- struct{}{}
	}
	return status, err
}

func ok() (int, error)      { return http.StatusOK, nil }
func refused() (int, error) { return 0, errors.New("connection refused") }

// TestWebhookDeliver covers the retry loop against a fake sender: a
// network error is retried with growing backoff and succeeds on a
// later attempt, and attempts stop once the endpoint's budget is spent.
func TestWebhookDeliver(t *testing.T) {
	tests := []struct {
		name         string
		retries      int
		results      []func() (int, error)
		wantAttempts int
		wantBackoffs []int
	}{
		{name: "first time", results: []func() (int, error){ok}, wantAttempts: 1},
		{name: "second time", results: []func() (int, error){refused, ok}, wantAttempts: 2, wantBackoffs: []int{1}},
		{name: "gives up", results: []func() (int, error){refused, refused, refused, ok}, wantAttempts: 3, wantBackoffs: []int{1, 2}},
		{name: "one retry", retries: 1, results: []func() (int, error){refused, refused, ok}, wantAttempts: 2, wantBackoffs: []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &fakeSender{results: tt.results}
			h := newWebhookHub(map[string]config.WebhookConfig{"ep": {URL: "http://ep", Retries: tt.retries}})
			var backoffs []int
			h.backoff = func(n int) time.Duration { backoffs = append(backoffs, n); return 0 }
			h.send = fs.send

			h.deliver(delivery{ep: h.endpoints[0], data: []byte("{}")})
			h.Close()

			if len(fs.attempts) != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", len(fs.attempts), tt.wantAttempts)
			}
			if !reflect.DeepEqual(backoffs, tt.wantBackoffs) {
				t.Errorf("backoffs = %v, want %v", backoffs, tt.wantBackoffs)
			}
		})
	}
}

// TestWebhookBroadcastMultipleEndpoints is a fan-out scenario: one
// event, two endpoints, each receives exactly one delivery.
func TestWebhookBroadcastMultipleEndpoints(t *testing.T) {
	fs := &fakeSender{results: []func() (int, error){ok, ok}, done: make(chan struct{}, 2)}
	h := newWebhookHub(map[string]config.WebhookConfig{"one": {URL: "http://one"}, "two": {URL: "http://two"}})
	defer h.Close()
	h.send = fs.send

	h.Broadcast(events.Event{Type: events.DocumentWritten, Timestamp: time.Now().UnixMilli()})
	for range 2 {
		select {
		case <-fs.done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for deliveries")
		}
	}
	h.Close()

	counts := map[string]int{}
	for _, name := range fs.attempts {
		counts[name]++
	}
	if counts["one"] != 1 || counts["two"] != 1 {
		t.Errorf("deliveries per endpoint = %v, want one each", counts)
	}
}

// TestWebhookQueueFull checks that Broadcast never blocks: with a
// single worker stuck on a slow endpoint and a queue of one, further
// events are dropped rather than queued or waited on.
func TestWebhookQueueFull(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	h := newWebhookHubSized(map[string]config.WebhookConfig{"slow": {URL: "http://slow"}}, 1, 1)
	h.send = func(webhookEndpoint, []byte) (int, error) {
		started <- struct{}{}
		<-release
		return http.StatusOK, nil
	}

	h.Broadcast(events.Event{Type: events.TaskCreated}) // taken by the worker
	<-started
	h.Broadcast(events.Event{Type: events.TaskCreated}) // fills the queue

	done := make(chan struct{})
	go func() {
		h.Broadcast(events.Event{Type: events.TaskCreated}) // must be dropped
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Broadcast blocked on a full queue")
	}
	if len(h.queue) != 1 {
		t.Errorf("queue length = %d, want 1", len(h.queue))
	}
	close(release)
	h.Close()
}

// TestWebhookCloseStopsRetry checks that Close does not wait out a
// backoff: a worker sleeping before a retry returns promptly.
func TestWebhookCloseStopsRetry(t *testing.T) {
	fs := &fakeSender{results: []func() (int, error){refused, refused, refused}, done: make(chan struct{}, 3)}
	h := newWebhookHub(map[string]config.WebhookConfig{"ep": {URL: "http://ep"}})
	h.backoff = func(int) time.Duration { return time.Hour }
	h.send = fs.send

	h.Broadcast(events.Event{Type: events.TaskCreated})
	<-fs.done // first attempt failed; worker now waits an hour

	closed := make(chan struct{})
	go func() { h.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waited for the backoff")
	}
	if len(fs.attempts) != 1 {
		t.Errorf("attempts after Close = %d, want 1", len(fs.attempts))
	}
}

// TestWebhookBroadcastUnmarshallable checks that an event whose
// metadata cannot be encoded is logged and dropped before any
// delivery is queued.
func TestWebhookBroadcastUnmarshallable(t *testing.T) {
	buf := captureLog(t)
	fs := &fakeSender{done: make(chan struct{}, 1)}
	h := newWebhookHub(map[string]config.WebhookConfig{"test": {URL: "http://test"}})
	h.send = fs.send

	h.Broadcast(events.Event{Type: events.TagAdded, Metadata: map[string]any{"fn": func() {}}})
	h.Close()

	if log := buf.String(); !strings.Contains(log, "webhook: marshalling event") {
		t.Errorf("Broadcast(unmarshallable) log = %q, want to contain %q", log, "webhook: marshalling event")
	}
	if len(fs.attempts) != 0 {
		t.Errorf("Broadcast(unmarshallable) made %d attempts, want none", len(fs.attempts))
	}
}
