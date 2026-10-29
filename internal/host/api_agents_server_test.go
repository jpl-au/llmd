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
		name    string
		addr    string
		want    string
		wantErr error
	}{
		{name: "server running", addr: running, want: srv.URL},
		{name: "server not running", addr: closed, wantErr: sdk.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			TestSetup(t, TestWorkdir)
			cfg := "server:\n  addr: " + tt.addr + "\n"
			if err := os.WriteFile(filepath.Join(".llmd", "config.yaml"), []byte(cfg), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := serverURL()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("serverURL() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "llmd serve") {
				t.Errorf("serverURL() error = %q, want it to name llmd serve", err)
			}
			if got != tt.want {
				t.Errorf("serverURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
