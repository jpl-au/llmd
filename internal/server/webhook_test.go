package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/pkg/events"
)

func TestNewWebhookHubNil(t *testing.T) {
	if h := newWebhookHub(nil); h != nil {
		t.Fatal("expected nil hub for nil config")
	}
	if h := newWebhookHub(map[string]config.WebhookConfig{}); h != nil {
		t.Fatal("expected nil hub for empty config")
	}
}

func TestWebhookBroadcast(t *testing.T) {
	var mu sync.Mutex
	var received []events.Event
	var gotAuth string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		gotAuth = r.Header.Get("Authorization")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading body: %v", err)
			return
		}

		var e events.Event
		if err := json.Unmarshal(body, &e); err != nil {
			t.Errorf("unmarshalling event: %v", err)
			return
		}
		received = append(received, e)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newWebhookHub(map[string]config.WebhookConfig{
		"test": {URL: ts.URL, Key: "secret"},
	})

	e := events.Event{
		Type:      events.AuditCreated,
		Path:      "docs/spec",
		Key:       "abc123",
		Author:    "gemini",
		Timestamp: time.Now().UnixMilli(),
	}
	h.Broadcast(e)

	// Broadcast fires goroutines — wait briefly for delivery.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("got %d events, want 1", len(received))
	}
	if received[0].Type != events.AuditCreated {
		t.Errorf("type = %q, want %q", received[0].Type, events.AuditCreated)
	}
	if received[0].Path != "docs/spec" {
		t.Errorf("path = %q, want %q", received[0].Path, "docs/spec")
	}
	if received[0].Author != "gemini" {
		t.Errorf("author = %q, want %q", received[0].Author, "gemini")
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("auth = %q, want %q", gotAuth, "Bearer secret")
	}
}

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

func TestWebhookBroadcastNoKey(t *testing.T) {
	var gotAuth string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newWebhookHub(map[string]config.WebhookConfig{
		"nokey": {URL: ts.URL},
	})

	h.Broadcast(events.Event{Type: events.TaskCreated, Timestamp: time.Now().UnixMilli()})
	time.Sleep(100 * time.Millisecond)

	if gotAuth != "" {
		t.Errorf("auth = %q, want empty (no key configured)", gotAuth)
	}
}

func TestWebhookBroadcastServerError(t *testing.T) {
	// Verify that a 500 response does not panic or block.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	h := newWebhookHub(map[string]config.WebhookConfig{
		"failing": {URL: ts.URL, Key: "x"},
	})

	h.Broadcast(events.Event{Type: events.AuditReplied, Timestamp: time.Now().UnixMilli()})
	time.Sleep(100 * time.Millisecond)
	// No panic, no hang — that's the test.
}

func TestWebhookBroadcastUnreachable(t *testing.T) {
	// Verify that an unreachable endpoint does not panic or block.
	h := newWebhookHub(map[string]config.WebhookConfig{
		"dead": {URL: "http://127.0.0.1:1", Key: "x"},
	})

	h.Broadcast(events.Event{Type: events.TaskMoved, Timestamp: time.Now().UnixMilli()})
	time.Sleep(200 * time.Millisecond)
	// No panic, no hang — that's the test.
}
