package resolve_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/resolve"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		path    string
		version *int
	}{
		{"notes/readme", "notes/readme", nil},
		{"notes/readme:3", "notes/readme", new(3)},
		{"my:file:5", "my:file", new(5)},
		{"notes/readme:abc", "notes/readme:abc", nil},
		{"0mnnqhmvm", "0mnnqhmvm", nil},
		{"0mnnqhmvm:7", "0mnnqhmvm", new(7)},
	}

	for _, tt := range tests {
		path, version := resolve.ParseVersion(tt.input)
		if path != tt.path {
			t.Errorf("ParseVersion(%q) path = %q, want %q", tt.input, path, tt.path)
		}
		if (version == nil) != (tt.version == nil) {
			t.Errorf("ParseVersion(%q) version nil = %v, want %v", tt.input, version == nil, tt.version == nil)
		} else if version != nil && *version != *tt.version {
			t.Errorf("ParseVersion(%q) version = %d, want %d", tt.input, *version, *tt.version)
		}
	}
}

func TestIdentifier_Path(t *testing.T) {
	ctx := context.Background()
	r := resolve.Identifier(ctx, "notes/readme", nil)
	if r.Path != "notes/readme" {
		t.Errorf("Path = %q, want %q", r.Path, "notes/readme")
	}
	if r.Version != nil {
		t.Errorf("Version = %v, want nil", r.Version)
	}
	if r.ByKey {
		t.Error("ByKey = true, want false")
	}
}

func TestIdentifier_PathVersion(t *testing.T) {
	ctx := context.Background()
	r := resolve.Identifier(ctx, "notes/readme:3", nil)
	if r.Path != "notes/readme" {
		t.Errorf("Path = %q, want %q", r.Path, "notes/readme")
	}
	if r.Version == nil || *r.Version != 3 {
		t.Errorf("Version = %v, want 3", r.Version)
	}
}

func TestIdentifier_Key(t *testing.T) {
	ctx := context.Background()
	lookup := func(_ context.Context, key string) (string, error) {
		if key == "0mnnqhmvm" {
			return "observations/llmd-recorder", nil
		}
		return "", errors.New("not found")
	}
	r := resolve.Identifier(ctx, "0mnnqhmvm", lookup)
	if r.Path != "observations/llmd-recorder" {
		t.Errorf("Path = %q, want %q", r.Path, "observations/llmd-recorder")
	}
	if r.ByKey != true {
		t.Error("ByKey = false, want true")
	}
}

func TestIdentifier_KeyVersion(t *testing.T) {
	ctx := context.Background()
	lookup := func(_ context.Context, key string) (string, error) {
		if key == "0mnnqhmvm" {
			return "observations/llmd-recorder", nil
		}
		return "", errors.New("not found")
	}
	r := resolve.Identifier(ctx, "0mnnqhmvm:7", lookup)
	if r.Path != "observations/llmd-recorder" {
		t.Errorf("Path = %q, want %q", r.Path, "observations/llmd-recorder")
	}
	if r.Version == nil || *r.Version != 7 {
		t.Errorf("Version = %v, want 7", r.Version)
	}
	if !r.ByKey {
		t.Error("ByKey = false, want true")
	}
}

func TestIdentifier_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	lookup := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("not found")
	}
	// Falls through to treating it as a path
	r := resolve.Identifier(ctx, "0mnnqhmvm", lookup)
	if r.Path != "0mnnqhmvm" {
		t.Errorf("Path = %q, want %q", r.Path, "0mnnqhmvm")
	}
	if r.ByKey {
		t.Error("ByKey = true, want false")
	}
}

//go:fix inline
func intPtr(v int) *int { return new(v) }
