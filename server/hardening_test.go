package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// lsRuntime is a stub with one read command and one write command
// that both succeed.
func lsRuntime() *stub {
	return &stub{
		cmds: map[string]*sdk.Command{
			"ls":    {Name: "ls"},
			"write": {Name: "write", NeedsAuthor: true},
		},
		dispatch: func(context.Context, string, []string, string, []byte, string) (sdk.Response, error) {
			return sdk.Text("ok"), nil
		},
	}
}

// TestLocal covers the DNS rebinding guard. Without a token only
// requests addressed to this machine, from pages served by it, are
// answered; with a token the token decides.
func TestLocal(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		host   string
		origin string
		want   int
	}{
		{name: "localhost", host: "localhost:5563", want: http.StatusOK},
		{name: "loopback v4", host: "127.0.0.1:5563", want: http.StatusOK},
		{name: "loopback v6", host: "[::1]:5563", want: http.StatusOK},
		{name: "localhost subdomain", host: "app.localhost", want: http.StatusOK},
		{name: "rebound domain", host: "rebind.example:5563", want: http.StatusForbidden},
		{name: "local page", host: "localhost:5563", origin: "http://localhost:3000", want: http.StatusOK},
		{name: "foreign page", host: "localhost:5563", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "unparsable origin", host: "localhost:5563", origin: "::", want: http.StatusForbidden},
		{name: "token lets any host through", token: "s3cret", host: "llmd.internal", origin: "https://evil.example", want: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(lsRuntime(), WithAddr("localhost:5563"), WithAuthor("test"), WithToken(tt.token))
			req := httptest.NewRequest(http.MethodGet, "/ls", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			s.mux.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("GET /ls Host %s Origin %q = %d, want %d (%s)", tt.host, tt.origin, rec.Code, tt.want, rec.Body)
			}
		})
	}
}

// TestMaxBody checks that a body over the limit is refused with 413
// before the command runs.
func TestMaxBody(t *testing.T) {
	tests := []struct {
		name string
		size int
		want int
	}{
		{name: "at the limit", size: 64, want: http.StatusOK},
		{name: "over the limit", size: 65, want: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(lsRuntime(), WithAddr("localhost:0"), WithAuthor("test"), WithMaxBody(64))
			req := httptest.NewRequest(http.MethodPost, "http://localhost/write/doc", strings.NewReader(strings.Repeat("x", tt.size)))
			rec := httptest.NewRecorder()
			s.mux.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("POST %d bytes = %d, want %d (%s)", tt.size, rec.Code, tt.want, rec.Body)
			}
		})
	}
}

// TestBuildArgsOptionLike checks that a URL value a command would read
// as a flag is refused, and that query flags come out in name order.
func TestBuildArgsOptionLike(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    []string
		wantErr bool
	}{
		{name: "path", target: "/cat/notes/a", want: []string{"notes/a"}},
		{name: "path that is a flag", target: "/cat/--all", wantErr: true},
		{name: "query that is a flag", target: "/cat?q=-n", wantErr: true},
		{name: "flags in name order", target: "/cat/a?z=1&a=2&m=true", want: []string{"a", "--a", "2", "--m", "--z", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			var got []string
			var err error
			mux.HandleFunc("GET /cat/{path...}", func(w http.ResponseWriter, r *http.Request) {
				got, err = buildArgs(r, []string{"{path}"})
			})
			mux.HandleFunc("GET /cat", func(w http.ResponseWriter, r *http.Request) {
				got, err = buildArgs(r, nil)
			})
			mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.target, nil))
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildArgs(%s) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
			if !tt.wantErr && strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildArgs(%s) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

// freeAddr returns a loopback address with a port nothing listens on.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// serveUntil runs ListenAndServe in the background and waits until it
// accepts connections. The returned cancel stops it; wait returns its
// error.
func serveUntil(t *testing.T, s *Server) (cancel func(), wait func() error) {
	t.Helper()
	ctx, stop := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe(ctx) }()
	for range 100 {
		if c, err := net.Dial("tcp", s.addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return stop, func() error {
		select {
		case err := <-errCh:
			return err
		case <-time.After(3 * time.Second):
			t.Fatal("server did not stop within 3 seconds")
			return nil
		}
	}
}

// TestShutdownWithStream checks that a connected /events client does
// not hold shutdown open: it once waited out the whole shutdown
// timeout and returned context.DeadlineExceeded.
func TestShutdownWithStream(t *testing.T) {
	s := New(lsRuntime(), WithAddr(freeAddr(t)), WithShutdownTimeout(2*time.Second))
	stop, wait := serveUntil(t, s)

	resp, err := http.Get("http://" + s.addr + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	start := time.Now()
	stop()
	if err := wait(); err != nil {
		t.Errorf("ListenAndServe() after shutdown = %v, want nil", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("shutdown took %s with a stream open, want well under the 2s timeout", d)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Errorf("reading the ended stream: %v", err)
	}
}

// TestSlowHeaders checks that a client sending headers too slowly is
// disconnected rather than holding the connection.
func TestSlowHeaders(t *testing.T) {
	s := New(lsRuntime(), WithAddr(freeAddr(t)), WithTimeouts(100*time.Millisecond, 0, 0))
	stop, wait := serveUntil(t, s)
	defer func() { stop(); _ = wait() }()

	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "GET /ls HTTP/1.1\r\nHost: localhost\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// The server gives up on the headers and closes the connection;
	// the read ends with EOF (or a 408 first) well before 2s.
	if _, err := io.ReadAll(conn); err != nil {
		t.Errorf("connection was still open after the header timeout: %v", err)
	}
}

// TestNonLoopbackNeedsToken checks that the server will not listen
// beyond this machine without a token: the Host check cannot protect
// an address others can reach.
func TestNonLoopbackNeedsToken(t *testing.T) {
	s := New(lsRuntime(), WithAddr("0.0.0.0:0"))
	err := s.ListenAndServe(t.Context())
	if err == nil || !strings.Contains(err.Error(), "server.token must be set") {
		t.Errorf("ListenAndServe() on 0.0.0.0 without a token = %v, want a refusal", err)
	}
}
