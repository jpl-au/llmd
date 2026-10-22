package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/pkg/events"
)

func TestNewWebhookHub(t *testing.T) {
	tests := []struct {
		name          string
		cfg           map[string]config.WebhookConfig
		wantNil       bool
		wantEndpoints int
	}{
		{name: "nil config", cfg: nil, wantNil: true},
		{name: "empty config", cfg: map[string]config.WebhookConfig{}, wantNil: true},
		{name: "one endpoint", cfg: map[string]config.WebhookConfig{"a": {URL: "http://a"}}, wantEndpoints: 1},
		{name: "two endpoints", cfg: map[string]config.WebhookConfig{"a": {URL: "http://a"}, "b": {URL: "http://b", Key: "k"}}, wantEndpoints: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newWebhookHub(tt.cfg)
			if (h == nil) != tt.wantNil {
				t.Fatalf("newWebhookHub(%v) = %v, want nil %v", tt.cfg, h, tt.wantNil)
			}
			if h != nil && len(h.endpoints) != tt.wantEndpoints {
				t.Errorf("newWebhookHub(%v) endpoints = %d, want %d", tt.cfg, len(h.endpoints), tt.wantEndpoints)
			}
		})
	}
}

// webhookRecorder captures what a webhook endpoint received.
type webhookRecorder struct {
	mu          sync.Mutex
	auth        string
	contentType string
	received    []events.Event
}

// handler returns an endpoint that records the request and responds
// with status.
func (r *webhookRecorder) handler(t *testing.T, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()

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

// count returns the number of events received so far.
func (r *webhookRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.received)
}

// TestWebhookBroadcast covers header and payload variants for a single
// endpoint: bearer auth when a key is configured, no auth header when
// it is not, and that a failing or unreachable endpoint neither panics
// nor blocks.
func TestWebhookBroadcast(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		status      int  // endpoint response status
		unreachable bool // use a closed port instead of a test server
		event       events.Event
		wantAuth    string
		wantEvents  int
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
			wantAuth:   "Bearer secret",
			wantEvents: 1,
		},
		{
			name:       "no key",
			status:     http.StatusOK,
			event:      events.Event{Type: events.TaskCreated, Timestamp: time.Now().UnixMilli()},
			wantAuth:   "",
			wantEvents: 1,
		},
		{
			// A 500 response is logged, never surfaced.
			name:       "server error",
			key:        "x",
			status:     http.StatusInternalServerError,
			event:      events.Event{Type: events.AuditReplied, Timestamp: time.Now().UnixMilli()},
			wantAuth:   "Bearer x",
			wantEvents: 1,
		},
		{
			// A refused connection is logged, never surfaced.
			name:        "unreachable",
			key:         "x",
			unreachable: true,
			event:       events.Event{Type: events.TaskMoved, Timestamp: time.Now().UnixMilli()},
			wantEvents:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &webhookRecorder{}
			url := "http://127.0.0.1:1"
			if !tt.unreachable {
				ts := httptest.NewServer(rec.handler(t, tt.status))
				defer ts.Close()
				url = ts.URL
			}

			h := newWebhookHub(map[string]config.WebhookConfig{
				"test": {URL: url, Key: tt.key},
			})
			h.Broadcast(tt.event)

			// Broadcast fires goroutines - wait for delivery. When nothing
			// is expected, wait long enough to catch a stray delivery.
			deadline := time.Now().Add(2 * time.Second)
			for rec.count() < tt.wantEvents && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if tt.wantEvents == 0 {
				time.Sleep(200 * time.Millisecond)
			}

			rec.mu.Lock()
			defer rec.mu.Unlock()

			if len(rec.received) != tt.wantEvents {
				t.Fatalf("Broadcast(%v) delivered %d events, want %d", tt.event, len(rec.received), tt.wantEvents)
			}
			if tt.wantEvents == 0 {
				return
			}
			if !reflect.DeepEqual(rec.received[0], tt.event) {
				t.Errorf("Broadcast(%v) delivered %+v, want %+v", tt.event, rec.received[0], tt.event)
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

// TestWebhookBroadcastMultipleEndpoints is a fan-out scenario: one
// event, two endpoints, each receives exactly one delivery.
func TestWebhookBroadcastMultipleEndpoints(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}

	handler := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			counts[name]++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})
	}

	ts1 := httptest.NewServer(handler("one"))
	defer ts1.Close()
	ts2 := httptest.NewServer(handler("two"))
	defer ts2.Close()

	h := newWebhookHub(map[string]config.WebhookConfig{
		"one": {URL: ts1.URL},
		"two": {URL: ts2.URL},
	})

	h.Broadcast(events.Event{Type: events.DocumentWritten, Timestamp: time.Now().UnixMilli()})
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if counts["one"] != 1 {
		t.Errorf("endpoint one got %d events, want 1", counts["one"])
	}
	if counts["two"] != 1 {
		t.Errorf("endpoint two got %d events, want 1", counts["two"])
	}
}
