package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

func stubSDK(t *testing.T) {
	t.Helper()

	origDispatch := sdk.Dispatch
	origAllCmds := sdk.AllCommands
	t.Cleanup(func() {
		sdk.Dispatch = origDispatch
		sdk.AllCommands = origAllCmds
	})

	sdk.AllCommands = func() map[string]*sdk.Command {
		return map[string]*sdk.Command{
			"ls": {Name: "ls", Desc: "list"},
		}
	}
	sdk.Dispatch = func(_ context.Context, cmd string, _ []string, _ string, _ []byte, _ string) (sdk.Response, error) {
		return sdk.Text("ok"), nil
	}
}

// newTestServer builds a Server from the stubbed SDK and serves its
// mux over httptest. The server is closed on cleanup.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := New(config.Config{Server: config.ServerConfig{Addr: "localhost:0"}, Author: "test"}, nil)
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)
	return ts
}

// captureLog replaces the default slog handler with one that writes
// to the returned buffer for the duration of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })
	return &buf
}

// TestListenAndServeShutdown is a lifecycle scenario: start, cancel,
// confirm graceful shutdown within the deadline.
func TestListenAndServeShutdown(t *testing.T) {
	stubSDK(t)

	s := New(config.Config{Server: config.ServerConfig{Addr: "localhost:0"}, Author: "test"}, nil)

	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(ctx)
	}()

	// Give the server a moment to start, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ListenAndServe returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down within 3 seconds")
	}
}

// TestLogMiddleware checks that each request is logged with its
// method, path and the status the handler actually wrote.
func TestLogMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		dispatch   error
		wantStatus int
		wantLog    []string
	}{
		{"ok", nil, http.StatusOK, []string{"method=GET", "path=/ls", "status=200"}},
		{"not found", sdk.ErrNotFound, http.StatusNotFound, []string{"method=GET", "path=/ls", "status=404"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubSDK(t)
			sdk.Dispatch = func(_ context.Context, _ string, _ []string, _ string, _ []byte, _ string) (sdk.Response, error) {
				if tt.dispatch != nil {
					return nil, tt.dispatch
				}
				return sdk.Text("ok"), nil
			}
			buf := captureLog(t)
			ts := newTestServer(t)

			resp, err := http.Get(ts.URL + "/ls")
			if err != nil {
				t.Fatalf("GET /ls: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("GET /ls status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			log := buf.String()
			for _, want := range tt.wantLog {
				if !strings.Contains(log, want) {
					t.Errorf("log missing %q:\n%s", want, log)
				}
			}
		})
	}
}

// TestDispatchErrorStatus covers the sdk sentinel error to HTTP status
// mapping, including wrapped errors and the 500 fallback. Every error
// must come back as a JSON {"error": ...} body.
func TestDispatchErrorStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", sdk.ErrNotFound, http.StatusNotFound},
		{"missing arg", sdk.ErrMissingArg, http.StatusBadRequest},
		{"invalid arg", sdk.ErrInvalidArg, http.StatusBadRequest},
		{"exists", sdk.ErrExists, http.StatusConflict},
		{"no spec", sdk.ErrNoSpec, http.StatusUnprocessableEntity},
		{"order violation", sdk.ErrOrderViolation, http.StatusConflict},
		{"cycle", sdk.ErrCycle, http.StatusConflict},
		{"wrapped not found", fmt.Errorf("ls: %w", sdk.ErrNotFound), http.StatusNotFound},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubSDK(t)
			sdk.Dispatch = func(_ context.Context, _ string, _ []string, _ string, _ []byte, _ string) (sdk.Response, error) {
				return nil, tt.err
			}
			ts := newTestServer(t)

			resp, err := http.Get(ts.URL + "/ls")
			if err != nil {
				t.Fatalf("GET /ls: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.want {
				t.Errorf("GET /ls with %v: status = %d, want %d", tt.err, resp.StatusCode, tt.want)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decoding error body: %v", err)
			}
			if body["error"] != tt.err.Error() {
				t.Errorf("error body = %q, want %q", body["error"], tt.err.Error())
			}
		})
	}
}

// TestRoutes covers the route and method mapping built by register:
// read commands are GET, mutations are POST, mixed commands accept
// both, skipped commands are not routed, and the URL path becomes the
// dispatched command's arguments.
func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCmd    string
		wantArgs   []string
	}{
		{"read get", "GET", "/cat/docs/api", http.StatusOK, "cat", []string{"docs/api"}},
		{"read get bare", "GET", "/cat", http.StatusOK, "cat", nil},
		{"read post rejected", "POST", "/cat/docs/api", http.StatusMethodNotAllowed, "", nil},
		{"write post", "POST", "/edit/docs/api", http.StatusOK, "edit", []string{"docs/api"}},
		{"write get rejected", "GET", "/edit/docs/api", http.StatusMethodNotAllowed, "", nil},
		{"mixed get splits path", "GET", "/task/show/abc", http.StatusOK, "task", []string{"show", "abc"}},
		{"mixed post splits path", "POST", "/task/move/abc/review", http.StatusOK, "task", []string{"move", "abc", "review"}},
		{"query positional", "GET", "/cat?q=hello", http.StatusOK, "cat", []string{"hello"}},
		{"query flag", "GET", "/cat/docs/api?version=3", http.StatusOK, "cat", []string{"docs/api", "--version", "3"}},
		{"skipped command", "GET", "/mcp", http.StatusNotFound, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubSDK(t)
			sdk.AllCommands = func() map[string]*sdk.Command {
				return map[string]*sdk.Command{
					"cat":  {Name: "cat"},
					"edit": {Name: "edit", NeedsAuthor: true},
					"task": {Name: "task"},
					"mcp":  {Name: "mcp"},
				}
			}
			var mu sync.Mutex
			var gotCmd string
			var gotArgs []string
			sdk.Dispatch = func(_ context.Context, cmd string, args []string, _ string, _ []byte, _ string) (sdk.Response, error) {
				mu.Lock()
				defer mu.Unlock()
				gotCmd, gotArgs = cmd, args
				return sdk.Text("ok"), nil
			}
			ts := newTestServer(t)

			req, err := http.NewRequestWithContext(t.Context(), tt.method, ts.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tt.method, tt.path, err)
			}
			resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("%s %s status = %d, want %d", tt.method, tt.path, resp.StatusCode, tt.wantStatus)
			}
			mu.Lock()
			defer mu.Unlock()
			if gotCmd != tt.wantCmd {
				t.Errorf("%s %s dispatched cmd = %q, want %q", tt.method, tt.path, gotCmd, tt.wantCmd)
			}
			if !slices.Equal(gotArgs, tt.wantArgs) {
				t.Errorf("%s %s dispatched args = %q, want %q", tt.method, tt.path, gotArgs, tt.wantArgs)
			}
		})
	}
}
