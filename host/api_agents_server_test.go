package host

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestServerURL covers the check spawn makes before creating anything:
// agents reach llmd through llmd serve, so a spawn without a running
// server is refused with a message that names the command to run.
func TestServerURL(t *testing.T) {
	// A port nothing listens on: bind one, then close it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	running := strings.TrimPrefix(srv.URL, "http://")

	tests := []struct {
		name     string
		addr     string
		token    string
		want     string
		wantAuth string
		wantErr  error
	}{
		{name: "server running", addr: running, want: srv.URL},
		{name: "token sent as a header", addr: running, token: "s3cret", want: srv.URL, wantAuth: " -H 'Authorization: Bearer s3cret'"},
		{name: "server not running", addr: closed, wantErr: sdk.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			TestSetup(t, TestWorkdir)
			cfg := "server:\n  addr: " + tt.addr + "\n"
			if tt.token != "" {
				cfg += "  token: " + tt.token + "\n"
			}
			if err := os.WriteFile(filepath.Join(".llmd", "config.yaml"), []byte(cfg), 0o644); err != nil {
				t.Fatal(err)
			}

			got, auth, err := serverURL()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("serverURL() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), sdk.Program()+" serve") {
				t.Errorf("serverURL() error = %q, want it to name %s serve", err, sdk.Program())
			}
			if got != tt.want || auth != tt.wantAuth {
				t.Errorf("serverURL() = %q, %q, want %q, %q", got, auth, tt.want, tt.wantAuth)
			}
		})
	}
}
