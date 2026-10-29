package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/host"
)

// TestServe covers the two cheap paths through serve: a malformed
// config is reported before anything listens, and a server bound to
// an ephemeral port shuts down cleanly when the command context is
// cancelled. Route behaviour belongs to the server package's own
// tests and the smoke suite.
func TestServe(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		cancel  bool // cancel the context shortly after starting
		wantErr string
	}{
		{
			name:    "malformed config",
			config:  "server: [oops\n",
			wantErr: "config",
		},
		{
			name:   "listens until cancelled",
			config: "server:\n  addr: \"127.0.0.1:0\"\n",
			cancel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host.TestHome(t)
			h := host.TestSetup(t, host.TestWorkdir)
			if err := os.WriteFile(filepath.Join(h.Dir(), ".llmd", "config.yaml"), []byte(tt.config), 0o644); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := h.Dispatch(ctx, "serve", nil, "", nil, "")
				done <- err
			}()
			if tt.cancel {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}

			select {
			case err := <-done:
				if tt.wantErr == "" && err != nil {
					t.Errorf("serve error = %v, want nil after cancellation", err)
				}
				if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
					t.Errorf("serve error = %v, want to contain %q", err, tt.wantErr)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("serve did not return")
			}
		})
	}
}
