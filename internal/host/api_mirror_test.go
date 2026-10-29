package host

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// mirrorHost changes into a fresh store directory: the mirror
// directory is relative to the working directory.
func mirrorHost(t *testing.T) (*Host, sdk.Context) {
	t.Helper()
	h := TestSetup(t, TestWorkdir)
	return h, h.Context(t.Context(), "alice")
}

func TestMirrorDirectory(t *testing.T) {
	_, c := mirrorHost(t)
	if got, want := c.Mirror.Directory(), filepath.Join(".llmd", "llmd"); got != want {
		t.Errorf("Directory() = %q, want %q", got, want)
	}
}

// TestMirrorPull walks a mirror through writes, a no-op repeat, a
// deletion and a change, checking the counts and the files each time.
func TestMirrorPull(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		steps  []struct {
			before func(t *testing.T, c sdk.Context)
			want   sdk.PullResult
			files  []string // relative .md files present afterwards
		}
	}{
		{
			name:   "all documents",
			prefix: "",
			steps: []struct {
				before func(t *testing.T, c sdk.Context)
				want   sdk.PullResult
				files  []string
			}{
				{
					before: func(t *testing.T, c sdk.Context) {
						writeDoc(t, c, "notes/one", "one")
						writeDoc(t, c, "notes/two", "two")
						writeDoc(t, c, "top", "top")
					},
					want:  sdk.PullResult{Wrote: 3},
					files: []string{"notes/one.md", "notes/two.md", "top.md"},
				},
				{
					want:  sdk.PullResult{Skipped: 3},
					files: []string{"notes/one.md", "notes/two.md", "top.md"},
				},
				{
					before: func(t *testing.T, c sdk.Context) {
						if err := c.Documents.Delete("notes/two", sdk.DeleteOpts{Author: "alice"}); err != nil {
							t.Fatal(err)
						}
						writeDoc(t, c, "top", "changed")
					},
					want:  sdk.PullResult{Wrote: 1, Skipped: 1, Removed: 1},
					files: []string{"notes/one.md", "top.md"},
				},
			},
		},
		{
			name:   "prefix",
			prefix: "notes/",
			steps: []struct {
				before func(t *testing.T, c sdk.Context)
				want   sdk.PullResult
				files  []string
			}{
				{
					before: func(t *testing.T, c sdk.Context) {
						writeDoc(t, c, "notes/one", "one")
						writeDoc(t, c, "other/skip", "skip")
					},
					want:  sdk.PullResult{Wrote: 1},
					files: []string{"notes/one.md"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := mirrorHost(t)
			dir := c.Mirror.Directory()
			for i, step := range tt.steps {
				if step.before != nil {
					step.before(t, c)
				}
				got, err := c.Mirror.Pull(tt.prefix, dir)
				if err != nil {
					t.Fatalf("step %d: Pull(%q, %q) error = %v", i, tt.prefix, dir, err)
				}
				if *got != step.want {
					t.Errorf("step %d: Pull(%q, %q) = %+v, want %+v", i, tt.prefix, dir, *got, step.want)
				}
				if files := mdFiles(t, dir); !slices.Equal(files, step.files) {
					t.Errorf("step %d: mirror files = %v, want %v", i, files, step.files)
				}
			}
			// Pulled content matches the store.
			data, err := os.ReadFile(filepath.Join(dir, "notes", "one.md"))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(data) != "one" {
				t.Errorf("notes/one.md = %q, want %q", data, "one")
			}
		})
	}
}

// TestMirrorPullError checks that a mirror directory that cannot be
// created is reported.
func TestMirrorPullError(t *testing.T) {
	_, c := mirrorHost(t)
	writeDoc(t, c, "doc", "x")
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Mirror.Pull("", blocker); err == nil {
		t.Errorf("Pull(\"\", %q) error = nil, want an error for a file in place of the directory", blocker)
	}
}

// mdFiles lists the .md files under dir, relative and sorted.
func mdFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".md" {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	slices.Sort(files)
	return files
}

// TestMirrorPush imports a directory of .md files, repeats it, edits a
// file and uses a prefix, checking which documents are created,
// updated and skipped.
func TestMirrorPush(t *testing.T) {
	type step struct {
		files map[string]string // written before the push
		opts  sdk.PushOpts
		want  sdk.PushResult
	}
	tests := []struct {
		name    string
		steps   []step
		wantDoc map[string]string // store content afterwards
		wantErr bool
	}{
		{
			name: "create, skip, update",
			steps: []step{
				{files: map[string]string{"one.md": "one", "nested/two.md": "two"}, want: sdk.PushResult{Created: []string{"nested/two", "one"}}},
				{want: sdk.PushResult{Skipped: []string{"nested/two", "one"}}},
				{files: map[string]string{"one.md": "changed"}, want: sdk.PushResult{Updated: []string{"one"}, Skipped: []string{"nested/two"}}},
			},
			wantDoc: map[string]string{"one": "changed", "nested/two": "two"},
		},
		{
			name: "prefix",
			steps: []step{
				// The prefix limits push to that part of the mirror, as
				// it does pull, so a pull and push round trip keeps the
				// paths. It once prefixed every file, making docs/docs/a.
				{files: map[string]string{"mirror/one.md": "one", "other/two.md": "two"}, opts: sdk.PushOpts{Prefix: "mirror/"}, want: sdk.PushResult{Created: []string{"mirror/one"}}},
			},
			wantDoc: map[string]string{"mirror/one": "one"},
		},
		{
			name:    "missing directory",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := mirrorHost(t)
			dir := c.Mirror.Directory()
			if tt.wantErr {
				if _, err := c.Mirror.Push(filepath.Join(dir, "missing"), sdk.PushOpts{}); err == nil {
					t.Errorf("Push(missing dir) error = nil, want an error")
				}
				return
			}
			for i, step := range tt.steps {
				for name, content := range step.files {
					path := filepath.Join(dir, filepath.FromSlash(name))
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				got, err := c.Mirror.Push(dir, step.opts)
				if err != nil {
					t.Fatalf("step %d: Push(%q, %+v) error = %v", i, dir, step.opts, err)
				}
				slices.Sort(got.Created)
				slices.Sort(got.Updated)
				slices.Sort(got.Skipped)
				if !slices.Equal(got.Created, step.want.Created) || !slices.Equal(got.Updated, step.want.Updated) || !slices.Equal(got.Skipped, step.want.Skipped) {
					t.Errorf("step %d: Push(%q, %+v) = %+v, want %+v", i, dir, step.opts, *got, step.want)
				}
			}
			for path, want := range tt.wantDoc {
				data, err := c.Documents.Read(path, sdk.ReadOpts{})
				if err != nil {
					t.Fatalf("Read(%q) error = %v", path, err)
				}
				if string(data) != want {
					t.Errorf("Read(%q) = %q, want %q", path, data, want)
				}
			}
		})
	}
}
