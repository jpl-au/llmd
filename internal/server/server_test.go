package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestListenAndServeShutdown(t *testing.T) {
	stubSDK(t)

	s := New("localhost:0", "test", nil)

	ctx, cancel := context.WithCancel(context.Background())

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

func TestLogMiddleware(t *testing.T) {
	stubSDK(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	s := New("localhost:0", "test", nil)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ls")
	if err != nil {
		t.Fatalf("GET /ls: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	log := buf.String()
	for _, want := range []string{"method=GET", "path=/ls", "status=200"} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q:\n%s", want, log)
		}
	}
}

func TestLogMiddlewareRecordsStatus(t *testing.T) {
	stubSDK(t)

	sdk.Dispatch = func(_ context.Context, _ string, _ []string, _ string, _ []byte, _ string) (sdk.Response, error) {
		return nil, sdk.ErrNotFound
	}

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	s := New("localhost:0", "test", nil)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ls")
	if err != nil {
		t.Fatalf("GET /ls: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	log := buf.String()
	if !strings.Contains(log, "status=404") {
		t.Errorf("log missing status=404:\n%s", log)
	}
}
