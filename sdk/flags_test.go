package sdk

import (
	"errors"
	"slices"
	"testing"
)

// TestParseArgs covers every accepted flag syntax. Each case parses one
// argument list against one flag set and checks the resulting values,
// presence, and positional arguments. Only the fields a case sets are
// asserted, so a case documents exactly what it is about.
func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		flags    []Flag
		args     []string
		wantBool map[string]bool
		wantStr  map[string]string
		wantInt  map[string]int
		wantHas  map[string]bool
		wantPos  []string
	}{
		{
			name:     "bool long",
			flags:    []Flag{{Name: "verbose", Type: "bool"}, {Name: "force", Type: "bool"}},
			args:     []string{"--verbose", "file.txt", "--force"},
			wantBool: map[string]bool{"verbose": true, "force": true},
			wantPos:  []string{"file.txt"},
		},
		{
			name:     "bool short",
			flags:    []Flag{{Name: "n", Type: "bool"}, {Name: "l", Type: "bool"}},
			args:     []string{"-n", "-l"},
			wantBool: map[string]bool{"n": true, "l": true},
		},
		{
			name:     "bool combined short",
			flags:    []Flag{{Name: "l", Type: "bool"}, {Name: "a", Type: "bool"}, {Name: "t", Type: "bool"}},
			args:     []string{"-lat"},
			wantBool: map[string]bool{"l": true, "a": true, "t": true},
		},
		{
			name:     "bool short alias",
			flags:    []Flag{{Name: "delete", Short: "d", Type: "bool"}, {Name: "find", Short: "f", Type: "bool"}},
			args:     []string{"-d"},
			wantBool: map[string]bool{"delete": true, "find": false},
		},
		{
			name:    "string long separate",
			flags:   []Flag{{Name: "message", Type: "string"}},
			args:    []string{"--message", "hello"},
			wantStr: map[string]string{"message": "hello"},
		},
		{
			name:    "string long equals",
			flags:   []Flag{{Name: "message", Type: "string"}},
			args:    []string{"--message=world"},
			wantStr: map[string]string{"message": "world"},
		},
		{
			name:    "int long separate",
			flags:   []Flag{{Name: "version", Type: "int"}},
			args:    []string{"--version", "3"},
			wantInt: map[string]int{"version": 3},
		},
		{
			name:    "int long equals",
			flags:   []Flag{{Name: "version", Type: "int"}},
			args:    []string{"--version=5"},
			wantInt: map[string]int{"version": 5},
		},
		{
			name:    "int short compact",
			flags:   []Flag{{Name: "C", Type: "int"}},
			args:    []string{"-C3"},
			wantInt: map[string]int{"C": 3},
		},
		{
			name:    "int short separate",
			flags:   []Flag{{Name: "C", Type: "int"}},
			args:    []string{"-C", "3"},
			wantInt: map[string]int{"C": 3},
		},
		{
			// -nC3: n is bool, C consumes "3".
			name:     "int short in combined",
			flags:    []Flag{{Name: "n", Type: "bool"}, {Name: "C", Type: "int"}},
			args:     []string{"-nC3"},
			wantBool: map[string]bool{"n": true},
			wantInt:  map[string]int{"C": 3},
		},
		{
			name:     "double hyphen terminator",
			flags:    []Flag{{Name: "n", Type: "bool"}},
			args:     []string{"-n", "--", "--not-a-flag", "file"},
			wantBool: map[string]bool{"n": true},
			wantPos:  []string{"--not-a-flag", "file"},
		},
		{
			name:     "empty args",
			flags:    []Flag{{Name: "n", Type: "bool"}},
			args:     nil,
			wantBool: map[string]bool{"n": false},
		},
		{
			name:     "no flags defined",
			flags:    nil,
			args:     []string{"a", "b", "c"},
			wantBool: map[string]bool{"anything": false},
			wantPos:  []string{"a", "b", "c"},
		},
		{
			// priority=0 must be distinguishable from unset.
			name:    "has distinguishes zero from unset",
			flags:   []Flag{{Name: "priority", Type: "int"}, {Name: "title", Type: "string"}},
			args:    []string{"--priority", "0"},
			wantHas: map[string]bool{"priority": true, "title": false},
			wantInt: map[string]int{"priority": 0},
		},
		{
			name:     "defaults",
			flags:    []Flag{{Name: "n", Type: "bool"}, {Name: "msg", Type: "string"}, {Name: "count", Type: "int"}},
			args:     nil,
			wantBool: map[string]bool{"n": false},
			wantStr:  map[string]string{"msg": ""},
			wantInt:  map[string]int{"count": 0},
		},
		{
			// Simulates: grep -nC3 pattern notes/
			name:     "mixed grep style",
			flags:    []Flag{{Name: "n", Type: "bool"}, {Name: "l", Type: "bool"}, {Name: "c", Type: "bool"}, {Name: "C", Type: "int"}},
			args:     []string{"-nC3", "pattern", "notes/"},
			wantBool: map[string]bool{"n": true},
			wantInt:  map[string]int{"C": 3},
			wantPos:  []string{"pattern", "notes/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fv, pos, err := ParseArgs(tt.flags, tt.args)
			if err != nil {
				t.Fatalf("ParseArgs(%v) error = %v", tt.args, err)
			}
			for name, want := range tt.wantBool {
				if got := fv.Bool(name); got != want {
					t.Errorf("Bool(%q) = %v, want %v", name, got, want)
				}
			}
			for name, want := range tt.wantStr {
				if got := fv.String(name); got != want {
					t.Errorf("String(%q) = %q, want %q", name, got, want)
				}
			}
			for name, want := range tt.wantInt {
				if got := fv.Int(name); got != want {
					t.Errorf("Int(%q) = %d, want %d", name, got, want)
				}
			}
			for name, want := range tt.wantHas {
				if got := fv.Has(name); got != want {
					t.Errorf("Has(%q) = %v, want %v", name, got, want)
				}
			}
			if !slices.Equal(pos, tt.wantPos) {
				t.Errorf("positional = %v, want %v", pos, tt.wantPos)
			}
		})
	}
}

// TestParseArgs_Errors covers argument lists that must be rejected,
// always as sdk.ErrInvalidArg so callers and HTTP can classify them.
func TestParseArgs_Errors(t *testing.T) {
	tests := []struct {
		name  string
		flags []Flag
		args  []string
	}{
		{"unknown long flag", []Flag{{Name: "n", Type: "bool"}}, []string{"--unknown"}},
		{"unknown short flag", []Flag{{Name: "n", Type: "bool"}}, []string{"-x"}},
		{"missing value", []Flag{{Name: "message", Type: "string"}}, []string{"--message"}},
		{"invalid int", []Flag{{Name: "version", Type: "int"}}, []string{"--version", "abc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ParseArgs(tt.flags, tt.args); !errors.Is(err, ErrInvalidArg) {
				t.Errorf("ParseArgs(%v) error = %v, want ErrInvalidArg", tt.args, err)
			}
		})
	}
}
