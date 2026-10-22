package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// Deletion times used by TestVacuum. t1 is 2001-09-09T01:46:40Z; t2
// is one day later. The clock is pinned so --older-than can be given
// as an exact RFC 3339 cutoff.
const (
	metaVacuumT1 = int64(1_000_000_000_000)
	metaVacuumT2 = metaVacuumT1 + 24*60*60*1000
)

// metaCutoff formats a Unix-millisecond instant as RFC 3339.
func metaCutoff(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// TestVacuum covers --dry-run, --older-than, the plain purge, the
// counts of tags and links purged with a document, orphaned tags on
// live documents, and the flag errors. Each case runs against a fresh
// store holding "a" (tagged, linked to "b"), "b" and "c"; seed then
// performs the deletions the case needs.
func TestVacuum(t *testing.T) {
	// deleteAC soft-deletes "a" at t1 and "c" at t2.
	deleteAC := func(t *testing.T, c sdk.Context) {
		clocktest.Fixed(t, metaVacuumT1)
		run(t, c, "rm", nil, "a")
		clocktest.Fixed(t, metaVacuumT2)
		run(t, c, "rm", nil, "c")
	}

	tests := []struct {
		name        string
		author      string
		seed        func(t *testing.T, c sdk.Context)
		args        []string
		wantText    []string
		wantAbsent  []string
		wantDocs    int64
		wantTags    int64
		wantLinks   int64
		wantPaths   []string
		wantDryRun  bool
		wantRestore []string // paths that must still be restorable afterwards
		wantPurged  []string // paths that must be gone for good afterwards
		wantErr     error
		wantAnyErr  bool
	}{
		{
			name:       "nothing to vacuum",
			author:     "alice",
			args:       nil,
			wantText:   []string{"Nothing to vacuum"},
			wantAbsent: []string{"Vacuumed", "Would vacuum"},
		},
		{
			name:       "dry run lists paths and changes nothing",
			author:     "alice",
			seed:       deleteAC,
			args:       []string{"--dry-run"},
			wantText:   []string{"Would vacuum 2 documents, 1 tags, 1 links", "\n  a\n", "\n  c"},
			wantAbsent: []string{"Vacuumed", "  b"},
			wantDocs:   2, wantTags: 1, wantLinks: 1,
			wantPaths:   []string{"a", "c"},
			wantDryRun:  true,
			wantRestore: []string{"a", "c"},
		},
		{
			name:        "short dry run flag",
			author:      "alice",
			seed:        deleteAC,
			args:        []string{"-n"},
			wantText:    []string{"Would vacuum 2 documents"},
			wantPaths:   []string{"a", "c"},
			wantDryRun:  true,
			wantRestore: []string{"a", "c"},
		},
		{
			name:       "purge everything deleted",
			author:     "alice",
			seed:       deleteAC,
			args:       nil,
			wantText:   []string{"Vacuumed 2 documents, 1 tags, 1 links", "\n  a\n", "\n  c"},
			wantAbsent: []string{"Would vacuum"},
			wantDocs:   2, wantTags: 1, wantLinks: 1,
			wantPaths:  []string{"a", "c"},
			wantPurged: []string{"a", "c"},
		},
		{
			name:       "older than keeps recent deletions",
			author:     "alice",
			seed:       deleteAC,
			args:       []string{"--older-than", metaCutoff(metaVacuumT1 + 12*60*60*1000)},
			wantText:   []string{"Vacuumed 1 documents, 1 tags, 1 links", "  a"},
			wantAbsent: []string{"  c"},
			wantDocs:   1, wantTags: 1, wantLinks: 1,
			wantPaths:   []string{"a"},
			wantPurged:  []string{"a"},
			wantRestore: []string{"c"},
		},
		{
			name:        "older than cutoff before every deletion",
			author:      "alice",
			seed:        deleteAC,
			args:        []string{"--older-than", metaCutoff(metaVacuumT1 - 1000)},
			wantText:    []string{"Nothing to vacuum"},
			wantRestore: []string{"a", "c"},
		},
		{
			name:       "older than cutoff after every deletion",
			author:     "alice",
			seed:       deleteAC,
			args:       []string{"--older-than", metaCutoff(metaVacuumT2 + 1000)},
			wantText:   []string{"Vacuumed 2 documents"},
			wantPaths:  []string{"a", "c"},
			wantPurged: []string{"a", "c"},
		},
		{
			name:        "dry run with older than",
			author:      "alice",
			seed:        deleteAC,
			args:        []string{"-n", "--older-than", metaCutoff(metaVacuumT1 + 12*60*60*1000)},
			wantText:    []string{"Would vacuum 1 documents", "  a"},
			wantAbsent:  []string{"  c"},
			wantPaths:   []string{"a"},
			wantDryRun:  true,
			wantRestore: []string{"a", "c"},
		},
		{
			// A tag removed from a live document is itself soft-deleted
			// and purged without touching the document.
			name:   "removed tag on live document",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "tag", nil, "-d", "a", "topic")
			},
			args:       nil,
			wantText:   []string{"Vacuumed 0 documents, 1 tags, 0 links"},
			wantAbsent: []string{"  a"},
			wantTags:   1,
			wantPaths:  nil,
		},
		{
			name:   "removed link on live documents",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "unlink", nil, "a", "b")
			},
			args:      nil,
			wantText:  []string{"Vacuumed 0 documents, 0 tags, 1 links"},
			wantLinks: 1,
		},
		{
			name:       "invalid older than",
			author:     "alice",
			args:       []string{"--older-than", "yesterday"},
			wantAnyErr: true,
		},
		{
			name:    "older than without value",
			author:  "alice",
			args:    []string{"--older-than"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "unknown flag",
			author:  "alice",
			args:    []string{"--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			// vacuum is not attributed to an author; it runs without one.
			name:       "without author",
			author:     "",
			seed:       deleteAC,
			args:       nil,
			wantText:   []string{"Vacuumed 2 documents"},
			wantPaths:  []string{"a", "c"},
			wantPurged: []string{"a", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clocktest.Fixed(t, metaVacuumT1)
			seed := newCLI(t, "alice")
			writeDoc(t, seed, "a", "# A")
			writeDoc(t, seed, "b", "# B")
			writeDoc(t, seed, "c", "# C")
			run(t, seed, "tag", nil, "a", "topic")
			run(t, seed, "link", nil, "a", "b")
			if tt.seed != nil {
				tt.seed(t, seed)
			}
			c := seed
			c.Author = tt.author

			if tt.wantErr != nil || tt.wantAnyErr {
				err := runErr(t, c, "vacuum", nil, tt.args...)
				if tt.wantAnyErr && err == nil {
					t.Errorf("vacuum %v error = nil, want an error", tt.args)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("vacuum %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "vacuum", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("vacuum %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("vacuum %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			r, ok := data(resp).(sdk.VacuumResult)
			if !ok {
				t.Fatalf("vacuum %v Data = %T, want sdk.VacuumResult", tt.args, data(resp))
			}
			if r.DryRun != tt.wantDryRun {
				t.Errorf("vacuum %v DryRun = %v, want %v", tt.args, r.DryRun, tt.wantDryRun)
			}
			if tt.wantDocs != 0 || tt.wantTags != 0 || tt.wantLinks != 0 {
				if r.Documents != tt.wantDocs || r.Tags != tt.wantTags || r.Links != tt.wantLinks {
					t.Errorf("vacuum %v counts = %d docs, %d tags, %d links, want %d, %d, %d",
						tt.args, r.Documents, r.Tags, r.Links, tt.wantDocs, tt.wantTags, tt.wantLinks)
				}
			}
			if strings.Join(r.Paths, ",") != strings.Join(tt.wantPaths, ",") {
				t.Errorf("vacuum %v Paths = %v, want %v", tt.args, r.Paths, tt.wantPaths)
			}
			// Documents still restorable come back; purged ones are gone.
			for _, p := range tt.wantRestore {
				if err := runErr(t, seed, "restore", nil, p); err != nil {
					t.Errorf("after vacuum %v, restore %s = %v, want success", tt.args, p, err)
				}
			}
			for _, p := range tt.wantPurged {
				if err := runErr(t, seed, "restore", nil, p); !errors.Is(err, sdk.ErrNotFound) {
					t.Errorf("after vacuum %v, restore %s = %v, want %v", tt.args, p, err, sdk.ErrNotFound)
				}
			}
			// Live documents are never touched.
			if ok, err := seed.Documents.Exists("b"); err != nil || !ok {
				t.Errorf("after vacuum %v, Exists(b) = %v, %v, want true", tt.args, ok, err)
			}
		})
	}
}

// TestVacuumIdempotent checks that a second vacuum after a purge finds
// nothing, and that the purge survives a dry run run first.
func TestVacuumIdempotent(t *testing.T) {
	clocktest.Fixed(t, metaVacuumT1)
	c := newCLI(t, "alice")
	writeDoc(t, c, "a", "# A")
	run(t, c, "rm", nil, "a")

	if got := text(run(t, c, "vacuum", nil, "-n")); !strings.HasPrefix(got, "Would vacuum 1 documents") {
		t.Errorf("vacuum -n = %q, want Would vacuum 1 documents", got)
	}
	if got := text(run(t, c, "vacuum", nil)); !strings.HasPrefix(got, "Vacuumed 1 documents") {
		t.Errorf("vacuum = %q, want Vacuumed 1 documents", got)
	}
	if got := text(run(t, c, "vacuum", nil)); got != "Nothing to vacuum" {
		t.Errorf("second vacuum = %q, want Nothing to vacuum", got)
	}
	r, _ := data(run(t, c, "vacuum", nil)).(sdk.VacuumResult)
	if r.Documents+r.Tags+r.Links != 0 || len(r.Paths) != 0 {
		t.Errorf("second vacuum Data = %+v, want zero counts", r)
	}
}
