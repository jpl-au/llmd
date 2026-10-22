package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestRead(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, s *llmd.Store)
		path        string
		version     int // 0 reads the latest version
		want        string
		wantVersion int
		wantMessage string
		wantErr     error
	}{
		{
			name: "latest version",
			setup: func(t *testing.T, s *llmd.Store) {
				opts := testWriteOpts()
				opts.Message = "initial"
				if _, err := s.Documents.Write(t.Context(), "docs/readme", "# Hello", opts); err != nil {
					t.Fatalf("Write: %v", err)
				}
			},
			path:        "docs/readme",
			want:        "# Hello",
			wantVersion: 1,
			wantMessage: "initial",
		},
		{
			name: "specific version",
			setup: func(t *testing.T, s *llmd.Store) {
				mustWrite(t, s, "docs/readme", "version 1")
				mustWrite(t, s, "docs/readme", "version 2")
				mustWrite(t, s, "docs/readme", "version 3")
			},
			path:        "docs/readme",
			version:     2,
			want:        "version 2",
			wantVersion: 2,
		},
		{
			name:    "not found",
			path:    "nonexistent",
			wantErr: documents.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, s)
			}

			var opts documents.ReadOptions
			if tt.version != 0 {
				v := tt.version
				opts.Version = &v
			}
			doc, err := s.Documents.Read(ctx, tt.path, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Read(%q, version %d) error = %v, want %v", tt.path, tt.version, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if doc.Content != tt.want {
				t.Errorf("Read(%q, version %d).Content = %q, want %q", tt.path, tt.version, doc.Content, tt.want)
			}
			if doc.Version != tt.wantVersion {
				t.Errorf("Read(%q, version %d).Version = %d, want %d", tt.path, tt.version, doc.Version, tt.wantVersion)
			}
			if doc.Message != tt.wantMessage {
				t.Errorf("Read(%q, version %d).Message = %q, want %q", tt.path, tt.version, doc.Message, tt.wantMessage)
			}
		})
	}
}

func TestReadByKey(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()

	written := mustWrite(t, s, "docs/readme", "content")

	doc, err := s.Documents.ReadByKey(ctx, written.Key)
	if err != nil {
		t.Fatalf("ReadByKey(%q) error = %v", written.Key, err)
	}
	if doc.Content != "content" {
		t.Errorf("ReadByKey(%q).Content = %q, want %q", written.Key, doc.Content, "content")
	}
}
