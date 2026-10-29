package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/config"
	"github.com/jpl-au/llmd/sdk"
)

// failWriter is a ResponseWriter whose body writes fail, the way a
// client that has gone away looks to the handler.
type failWriter struct {
	http.ResponseWriter
}

var errClientGone = errors.New("client gone")

func (failWriter) Write([]byte) (int, error) { return 0, errClientGone }

// TestWriteFailuresAreLogged checks that each writer logs, rather than
// panics or hides, a failure to write the body to the client.
func TestWriteFailuresAreLogged(t *testing.T) {
	tests := []struct {
		name    string
		write   func(w http.ResponseWriter)
		wantLog string
	}{
		{"text", func(w http.ResponseWriter) { writeResponse(w, plainReq(), sdk.Text("hi")) }, "writing text response"},
		{"markdown", func(w http.ResponseWriter) { writeResponse(w, plainReq(), sdk.Markdown{Text: "# hi"}) }, "writing text response"},
		{"result text", func(w http.ResponseWriter) { writeResponse(w, plainReq(), sdk.Result{Text: "hi", Data: 1}) }, "writing text response"},
		{"json", func(w http.ResponseWriter) { writeJSON(w, http.StatusOK, map[string]int{"n": 1}) }, "writing JSON response"},
		{"error", func(w http.ResponseWriter) { writeError(w, http.StatusBadRequest, errors.New("bad")) }, "writing error response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)
			rec := httptest.NewRecorder()
			tt.write(failWriter{rec})
			if got := buf.String(); !strings.Contains(got, tt.wantLog) || !strings.Contains(got, errClientGone.Error()) {
				t.Errorf("log = %q, want to contain %q and %q", got, tt.wantLog, errClientGone)
			}
		})
	}
}

// plainReq is a request without an Output header, so text responses
// are written as text.
func plainReq() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }

// errReader fails on the first read.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("broken body") }

// TestHandleBodyReadError checks that a request whose body cannot be
// read gets a 400 and is never dispatched.
func TestHandleBodyReadError(t *testing.T) {
	dispatched := false
	rt := &stub{
		cmds: map[string]*sdk.Command{"write": {Name: "write", NeedsAuthor: true}},
		dispatch: func(context.Context, string, []string, string, []byte, string) (sdk.Response, error) {
			dispatched = true
			return sdk.Text("ok"), nil
		},
	}
	s := New(rt, WithAddr("localhost:0"), WithAuthor("test"))

	req := httptest.NewRequest(http.MethodPost, "http://localhost/write/docs/a", errReader{})
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "broken body") {
		t.Errorf("body = %q, want the read error", rec.Body.String())
	}
	if dispatched {
		t.Error("command was dispatched despite the body read failing")
	}
}

// TestDispatchErrorStatusAgents covers the agent and readiness
// sentinels, which all answer 409.
func TestDispatchErrorStatusAgents(t *testing.T) {
	for _, err := range []error{sdk.ErrAgentRunning, sdk.ErrAgentNotRunning, sdk.ErrNotReady} {
		rec := httptest.NewRecorder()
		writeDispatchError(rec, err)
		if rec.Code != http.StatusConflict {
			t.Errorf("writeDispatchError(%v) status = %d, want %d", err, rec.Code, http.StatusConflict)
		}
	}
}

// TestListenAndServeClosesWebhooks checks that shutting the server
// down stops the webhook workers.
func TestListenAndServeClosesWebhooks(t *testing.T) {
	s := New(stubRuntime(), WithAddr("localhost:0"), WithAuthor("test"),
		WithWebhooks(map[string]config.WebhookConfig{"a": {URL: "http://127.0.0.1:1"}}))
	if s.webhook == nil {
		t.Fatal("webhook hub not created")
	}

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe(ctx) }()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ListenAndServe error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
	select {
	case <-s.webhook.done:
	default:
		t.Error("webhook hub still running after shutdown")
	}
}

// TestPostWebhookNetworkError checks that a refused connection comes
// back as an error with no status, which the hub treats as retryable.
func TestPostWebhookNetworkError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + ln.Addr().String()
	if err := ln.Close(); err != nil { // nothing listens here now
		t.Fatal(err)
	}

	status, err := postWebhook(webhookEndpoint{name: "a", url: url, timeout: time.Second}, []byte("{}"))
	if err == nil {
		t.Fatalf("postWebhook(%s) error = nil, want a connection error", url)
	}
	if status != 0 {
		t.Errorf("postWebhook(%s) status = %d, want 0", url, status)
	}
}

// TestWebhookQueueOption checks that the worker count and queue depth
// options reach the hub, and that zero or negative values keep the
// defaults.
func TestWebhookQueueOption(t *testing.T) {
	tests := []struct {
		name           string
		workers, depth int
		wantDepth      int
	}{
		{"defaults", 0, 0, webhookQueue},
		{"custom", 1, 7, 7},
		{"negative keeps defaults", -1, -1, webhookQueue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(stubRuntime(), WithWebhooks(map[string]config.WebhookConfig{"a": {URL: "http://a"}}),
				WithWebhookQueue(tt.workers, tt.depth))
			defer s.webhook.Close()
			if got := cap(s.webhook.queue); got != tt.wantDepth {
				t.Errorf("queue depth = %d, want %d", got, tt.wantDepth)
			}
		})
	}
}
