package resolve_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/resolve"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPath    string
		wantVersion *int
	}{
		{"path", "notes/readme", "notes/readme", nil},
		{"path with version", "notes/readme:3", "notes/readme", new(3)},
		{"colon in path", "my:file:5", "my:file", new(5)},
		{"non-numeric suffix", "notes/readme:abc", "notes/readme:abc", nil},
		{"key", "0mnnqhmvm1", "0mnnqhmvm1", nil},
		{"key with version", "0mnnqhmvm1:7", "0mnnqhmvm1", new(7)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, version := resolve.ParseVersion(tt.input)
			if path != tt.wantPath {
				t.Errorf("ParseVersion(%q) path = %q, want %q", tt.input, path, tt.wantPath)
			}
			if (version == nil) != (tt.wantVersion == nil) {
				t.Errorf("ParseVersion(%q) version nil = %v, want %v", tt.input, version == nil, tt.wantVersion == nil)
			} else if version != nil && *version != *tt.wantVersion {
				t.Errorf("ParseVersion(%q) version = %d, want %d", tt.input, *version, *tt.wantVersion)
			}
		})
	}
}

func TestIdentifier(t *testing.T) {
	lookup := func(_ context.Context, key string) (string, error) {
		if key == "0mnnqhmvm1" {
			return "observations/llmd-recorder", nil
		}
		return "", errors.New("not found")
	}
	notFound := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("not found")
	}

	tests := []struct {
		name        string
		value       string
		lookup      resolve.KeyToPath
		wantPath    string
		wantVersion *int
		wantByKey   bool
	}{
		{"path", "notes/readme", nil, "notes/readme", nil, false},
		{"path with version", "notes/readme:3", nil, "notes/readme", new(3), false},
		{"key", "0mnnqhmvm1", lookup, "observations/llmd-recorder", nil, true},
		{"key with version", "0mnnqhmvm1:7", lookup, "observations/llmd-recorder", new(7), true},
		{"key not found", "0mnnqhmvm1", notFound, "0mnnqhmvm1", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, version, byKey := resolve.Identifier(t.Context(), tt.value, tt.lookup)
			if path != tt.wantPath {
				t.Errorf("Identifier(%q) path = %q, want %q", tt.value, path, tt.wantPath)
			}
			if (version == nil) != (tt.wantVersion == nil) {
				t.Errorf("Identifier(%q) version nil = %v, want %v", tt.value, version == nil, tt.wantVersion == nil)
			} else if version != nil && *version != *tt.wantVersion {
				t.Errorf("Identifier(%q) version = %d, want %d", tt.value, *version, *tt.wantVersion)
			}
			if byKey != tt.wantByKey {
				t.Errorf("Identifier(%q) byKey = %v, want %v", tt.value, byKey, tt.wantByKey)
			}
		})
	}
}
