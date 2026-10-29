package sqlite

import (
	"path/filepath"
	"testing"
	"time"
)

// TestOpen checks that the settings reach the connections: the tunable
// ones from Settings, and the fixed ones every store relies on.
func TestOpen(t *testing.T) {
	off := 0
	tests := []struct {
		name     string
		settings Settings
		want     map[string]int64
	}{
		{
			name:     "defaults",
			settings: Settings{},
			want: map[string]int64{
				"busy_timeout": 5000, "cache_size": -64 * 1024, "mmap_size": 256 << 20,
				"foreign_keys": 1, "page_size": 8192,
			},
		},
		{
			name:     "tuned",
			settings: Settings{BusyTimeout: 7 * time.Second, CacheMB: 8, MMapMB: &off},
			want:     map[string]int64{"busy_timeout": 7000, "cache_size": -8 * 1024, "mmap_size": 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(filepath.Join(t.TempDir(), "test.db"), tt.settings)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			// Create the file so file-level settings are fixed.
			if _, err := db.Query(`CREATE TABLE t (x)`).Exec(); err != nil {
				t.Fatal(err)
			}
			for pragma, want := range tt.want {
				row, err := db.Query("PRAGMA " + pragma).ReadRow()
				if err != nil {
					t.Fatalf("PRAGMA %s: %v", pragma, err)
				}
				var got int64
				if err := row.Scan(&got); err != nil {
					t.Fatalf("PRAGMA %s: %v", pragma, err)
				}
				if got != want {
					t.Errorf("PRAGMA %s = %d, want %d", pragma, got, want)
				}
			}
			row, err := db.Query("PRAGMA journal_mode").ReadRow()
			if err != nil {
				t.Fatal(err)
			}
			var mode string
			if err := row.Scan(&mode); err != nil || mode != "wal" {
				t.Errorf("PRAGMA journal_mode = %q, %v, want wal", mode, err)
			}
		})
	}
}
