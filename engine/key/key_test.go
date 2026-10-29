package key

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

func TestGenerate(t *testing.T) {
	k := Generate()
	if len(k) != Length {
		t.Errorf("Generate() = %q, want %d characters", k, Length)
	}
	if err := Validate(k); err != nil {
		t.Errorf("Generate() produced invalid key: %v", err)
	}
}

func TestGenerateAt(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero", 0, "0000000000"},
		{"one", 1, "00000000sg"},
		{"36", 36, "0000000sg0"},
		{"1000", 1000, "000000ly4g"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateAt(tt.ms)
			if got != tt.want {
				t.Errorf("GenerateAt(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestGenerateAtLength(t *testing.T) {
	// All keys should be 10 characters regardless of input, up to the
	// year 2083 (3.5e12 ms).
	tests := []int64{0, 1, 100, 10000, 1000000000000, 2000000000000, 3500000000000}
	for _, ms := range tests {
		got := GenerateAt(ms)
		if len(got) != Length {
			t.Errorf("GenerateAt(%d) = %q, length = %d, want %d", ms, got, len(got), Length)
		}
		if err := Validate(got); err != nil {
			t.Errorf("GenerateAt(%d) = %q, validation failed: %v", ms, got, err)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{"valid lowercase", "k7v3x8m2a1", nil},
		{"valid digits", "1234567890", nil},
		{"valid mixed", "abc123xyz0", nil},
		{"valid zeros", "0000000000", nil},

		{"too short", "k7v3x8m2a", ErrInvalidLength},
		{"too long", "k7v3x8m2abc", ErrInvalidLength},
		{"empty", "", ErrInvalidLength},

		{"uppercase", "K7V3X8M2A1", ErrInvalidChar},
		{"uppercase mixed", "k7v3x8m2aA", ErrInvalidChar},
		{"special char", "k7v3x8m2a!", ErrInvalidChar},
		{"space", "k7v3x8m2a ", ErrInvalidChar},
		{"underscore", "k7v3_8m2a1", ErrInvalidChar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.key)
			if err != tt.wantErr {
				t.Errorf("Validate(%q) = %v, want %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAtUniqueness(t *testing.T) {
	// Different timestamps should produce different keys
	seen := make(map[string]bool)
	for ms := range int64(1000) {
		k := GenerateAt(ms)
		if seen[k] {
			t.Errorf("GenerateAt(%d) produced duplicate key: %s", ms, k)
		}
		seen[k] = true
	}
}

func TestGenerateAtMonotonic(t *testing.T) {
	// Keys should be lexicographically sorted when timestamps increase
	prev := GenerateAt(0)
	for ms := int64(1); ms < 100; ms++ {
		curr := GenerateAt(ms)
		if curr <= prev {
			t.Errorf("GenerateAt(%d) = %q <= GenerateAt(%d) = %q, want monotonically increasing",
				ms, curr, ms-1, prev)
		}
		prev = curr
	}
}

func TestGenerateUniqueness(t *testing.T) {
	// Generate() should produce unique keys even in tight loops
	seen := make(map[string]bool)
	for i := range 1000 {
		k := Generate()
		if seen[k] {
			t.Errorf("Generate() produced duplicate key on iteration %d: %s", i, k)
		}
		seen[k] = true
	}
}

// TestForce covers the test hook: forced keys come out first, in order,
// then normal generation resumes.
func TestForce(t *testing.T) {
	t.Cleanup(func() { Force() })
	Force("aaaaaaaaa1", "aaaaaaaaa2")
	for _, want := range []string{"aaaaaaaaa1", "aaaaaaaaa2"} {
		if got := Generate(); got != want {
			t.Errorf("Generate() = %q, want forced %q", got, want)
		}
	}
	if got := Generate(); got == "aaaaaaaaa2" || Validate(got) != nil {
		t.Errorf("Generate() after forced keys = %q, want a fresh valid key", got)
	}
}

// TestInsert covers the retry: a taken key is replaced and the insert
// run again, any other error comes straight back, and a run of taken
// keys gives up.
func TestInsert(t *testing.T) {
	taken := errors.New("constraint failed: UNIQUE constraint failed: tasks.key (2067)")
	locked := errors.New("database is locked")
	tests := []struct {
		name      string
		fail      []error // what the insert returns, call by call, then nil
		wantCalls int
		wantErr   error
	}{
		{name: "first key accepted", wantCalls: 1},
		{name: "taken key is replaced", fail: []error{taken}, wantCalls: 2},
		{name: "other error is returned", fail: []error{locked}, wantCalls: 1, wantErr: locked},
		{name: "gives up after a run of taken keys", fail: []error{taken, taken, taken, taken, taken}, wantCalls: attempts, wantErr: taken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			seen := map[string]bool{}
			var last string
			k, err := Insert("tasks.key", func(k string) error {
				calls++
				seen[k] = true
				last = k
				if calls <= len(tt.fail) {
					return tt.fail[calls-1]
				}
				return nil
			})
			if calls != tt.wantCalls {
				t.Errorf("insert ran %d times, want %d", calls, tt.wantCalls)
			}
			if len(seen) != calls {
				t.Errorf("Insert reused a key: %d distinct keys over %d calls", len(seen), calls)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Insert() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && k != last {
				t.Errorf("Insert() = %q, want the key of the accepted insert %q", k, last)
			}
			if tt.wantErr != nil && k != "" {
				t.Errorf("Insert() = %q on error, want empty", k)
			}
		})
	}
}

// TestTaken covers recognising SQLite's refusal for one column.
func TestTaken(t *testing.T) {
	taken := errors.New("constraint failed: UNIQUE constraint failed: messages.key (2067)")
	tests := []struct {
		name   string
		err    error
		column string
		want   bool
	}{
		{"matching column", taken, "messages.key", true},
		{"wrapped", fmt.Errorf("inserting: %w", taken), "messages.key", true},
		{"other column", taken, "messages.source_key", false},
		{"other error", errors.New("database is locked"), "messages.key", false},
		{"nil", nil, "messages.key", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Taken(tt.err, tt.column); got != tt.want {
				t.Errorf("Taken(%v, %q) = %v, want %v", tt.err, tt.column, got, tt.want)
			}
		})
	}
}

// TestGenerateCarriesStamp covers the per-process stamp: every key this
// process makes ends in the same 10 bits, so keys from another process
// with another stamp cannot be equal.
func TestGenerateCarriesStamp(t *testing.T) {
	for range 100 {
		v, err := strconv.ParseInt(Generate(), 36, 64)
		if err != nil {
			t.Fatal(err)
		}
		if v%stamps != stamp {
			t.Fatalf("key value %d has stamp %d, want this process's %d", v, v%stamps, stamp)
		}
	}
}
