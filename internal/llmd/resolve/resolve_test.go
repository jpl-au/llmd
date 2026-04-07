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
	path, version, byKey := resolve.Identifier(ctx, "notes/readme", nil)
	if path != "notes/readme" {
		t.Errorf("path = %q, want %q", path, "notes/readme")
	}
	if version != nil {
		t.Errorf("version = %v, want nil", version)
	}
	if byKey {
		t.Error("byKey = true, want false")
	}
}

func TestIdentifier_PathVersion(t *testing.T) {
	ctx := context.Background()
	path, version, _ := resolve.Identifier(ctx, "notes/readme:3", nil)
	if path != "notes/readme" {
		t.Errorf("path = %q, want %q", path, "notes/readme")
	}
	if version == nil || *version != 3 {
		t.Errorf("version = %v, want 3", version)
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
	path, _, byKey := resolve.Identifier(ctx, "0mnnqhmvm", lookup)
	if path != "observations/llmd-recorder" {
		t.Errorf("path = %q, want %q", path, "observations/llmd-recorder")
	}
	if !byKey {
		t.Error("byKey = false, want true")
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
	path, version, byKey := resolve.Identifier(ctx, "0mnnqhmvm:7", lookup)
	if path != "observations/llmd-recorder" {
		t.Errorf("path = %q, want %q", path, "observations/llmd-recorder")
	}
	if version == nil || *version != 7 {
		t.Errorf("version = %v, want 7", version)
	}
	if !byKey {
		t.Error("byKey = false, want true")
	}
}

func TestIdentifier_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	lookup := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("not found")
	}
	path, _, byKey := resolve.Identifier(ctx, "0mnnqhmvm", lookup)
	if path != "0mnnqhmvm" {
		t.Errorf("path = %q, want %q", path, "0mnnqhmvm")
	}
	if byKey {
		t.Error("byKey = true, want false")
	}
}

//go:fix inline
func intPtr(v int) *int { return new(v) }
