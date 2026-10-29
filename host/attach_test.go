package host

import (
	"os"
	"testing"
)

// TestAttachAfterInit covers a host started without a store, as the
// MCP and HTTP servers are when no .llmd exists yet: init through the
// host attaches the store, later calls on the same host work, and the
// host value is not replaced.
func TestAttachAfterInit(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	h := New()
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Errorf("closing host: %v", err)
		}
	})
	if h.Dir() != "" {
		t.Fatalf("Dir() before init = %q, want empty", h.Dir())
	}

	if _, err := h.Exec(t.Context(), "init", nil, "", nil, ""); err != nil {
		t.Fatalf("init: %v", err)
	}
	if h.Dir() == "" {
		t.Fatalf("Dir() after init is empty: the store was not attached")
	}

	if _, err := h.Exec(t.Context(), "write", []string{"docs/a"}, "alice", []byte("# A"), ""); err != nil {
		t.Fatalf("write after init: %v", err)
	}
	resp, err := h.Exec(t.Context(), "cat", []string{"docs/a"}, "", nil, "")
	if err != nil {
		t.Fatalf("cat after init: %v", err)
	}
	if got := text(resp); got != "# A" {
		t.Errorf("cat after init = %q, want %q", got, "# A")
	}

	// A second attach is a no-op: the first store stays.
	h.attach(h.current())
	if h.Dir() == "" {
		t.Errorf("Dir() after a second attach is empty")
	}
}
