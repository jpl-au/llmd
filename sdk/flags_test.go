package sdk

import (
	"testing"
)

func TestParseBoolLong(t *testing.T) {
	flags := []Flag{
		{Name: "verbose", Type: "bool"},
		{Name: "force", Type: "bool"},
	}
	fv, pos, err := ParseArgs(flags, []string{"--verbose", "file.txt", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("verbose") {
		t.Error("expected verbose=true")
	}
	if !fv.Bool("force") {
		t.Error("expected force=true")
	}
	if len(pos) != 1 || pos[0] != "file.txt" {
		t.Errorf("unexpected positional: %v", pos)
	}
}

func TestParseBoolShort(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
		{Name: "l", Type: "bool"},
	}
	fv, _, err := ParseArgs(flags, []string{"-n", "-l"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("n") || !fv.Bool("l") {
		t.Error("expected both flags set")
	}
}

func TestParseBoolCombined(t *testing.T) {
	flags := []Flag{
		{Name: "l", Type: "bool"},
		{Name: "a", Type: "bool"},
		{Name: "t", Type: "bool"},
	}
	fv, _, err := ParseArgs(flags, []string{"-lat"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("l") || !fv.Bool("a") || !fv.Bool("t") {
		t.Error("expected all three flags set")
	}
}

func TestParseBoolShortAlias(t *testing.T) {
	flags := []Flag{
		{Name: "delete", Short: "d", Type: "bool"},
		{Name: "find", Short: "f", Type: "bool"},
	}
	fv, _, err := ParseArgs(flags, []string{"-d"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("delete") {
		t.Error("expected delete=true via -d")
	}
	if fv.Bool("find") {
		t.Error("expected find=false")
	}
}

func TestParseStringLong(t *testing.T) {
	flags := []Flag{
		{Name: "message", Type: "string"},
	}

	// --name value form
	fv, _, err := ParseArgs(flags, []string{"--message", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.String("message") != "hello" {
		t.Errorf("got %q, want %q", fv.String("message"), "hello")
	}

	// --name=value form
	fv, _, err = ParseArgs(flags, []string{"--message=world"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.String("message") != "world" {
		t.Errorf("got %q, want %q", fv.String("message"), "world")
	}
}

func TestParseIntLong(t *testing.T) {
	flags := []Flag{
		{Name: "version", Type: "int"},
	}

	// --name value form
	fv, _, err := ParseArgs(flags, []string{"--version", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.Int("version") != 3 {
		t.Errorf("got %d, want 3", fv.Int("version"))
	}

	// --name=value form
	fv, _, err = ParseArgs(flags, []string{"--version=5"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.Int("version") != 5 {
		t.Errorf("got %d, want 5", fv.Int("version"))
	}
}

func TestParseIntShortCompact(t *testing.T) {
	flags := []Flag{
		{Name: "C", Type: "int"},
	}

	// -C3 compact form
	fv, _, err := ParseArgs(flags, []string{"-C3"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.Int("C") != 3 {
		t.Errorf("got %d, want 3", fv.Int("C"))
	}

	// -C 3 separate form
	fv, _, err = ParseArgs(flags, []string{"-C", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.Int("C") != 3 {
		t.Errorf("got %d, want 3", fv.Int("C"))
	}
}

func TestParseIntShortInCombined(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
		{Name: "C", Type: "int"},
	}
	// -nC3: n is bool, C consumes "3"
	fv, _, err := ParseArgs(flags, []string{"-nC3"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("n") {
		t.Error("expected n=true")
	}
	if fv.Int("C") != 3 {
		t.Errorf("got C=%d, want 3", fv.Int("C"))
	}
}

func TestParseDoubleHyphenTerminator(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
	}
	fv, pos, err := ParseArgs(flags, []string{"-n", "--", "--not-a-flag", "file"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("n") {
		t.Error("expected n=true")
	}
	if len(pos) != 2 || pos[0] != "--not-a-flag" || pos[1] != "file" {
		t.Errorf("unexpected positional: %v", pos)
	}
}

func TestParseUnknownLongFlag(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
	}
	_, _, err := ParseArgs(flags, []string{"--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseUnknownShortFlag(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
	}
	_, _, err := ParseArgs(flags, []string{"-x"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseMissingValue(t *testing.T) {
	flags := []Flag{
		{Name: "message", Type: "string"},
	}
	_, _, err := ParseArgs(flags, []string{"--message"})
	if err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestParseInvalidInt(t *testing.T) {
	flags := []Flag{
		{Name: "version", Type: "int"},
	}
	_, _, err := ParseArgs(flags, []string{"--version", "abc"})
	if err == nil {
		t.Fatal("expected error for invalid integer")
	}
}

func TestParseEmptyArgs(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
	}
	fv, pos, err := ParseArgs(flags, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Bool("n") {
		t.Error("expected n=false")
	}
	if len(pos) != 0 {
		t.Errorf("expected no positional, got %v", pos)
	}
}

func TestParseNoFlags(t *testing.T) {
	fv, pos, err := ParseArgs(nil, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if fv.Bool("anything") {
		t.Error("expected false for unset flag")
	}
	if len(pos) != 3 {
		t.Errorf("expected 3 positional, got %d", len(pos))
	}
}

func TestParseHas(t *testing.T) {
	flags := []Flag{
		{Name: "priority", Type: "int"},
		{Name: "title", Type: "string"},
	}
	fv, _, err := ParseArgs(flags, []string{"--priority", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Has("priority") {
		t.Error("expected Has(priority)=true")
	}
	if fv.Has("title") {
		t.Error("expected Has(title)=false")
	}
	// priority=0 should be distinguishable from unset
	if fv.Int("priority") != 0 {
		t.Errorf("got %d, want 0", fv.Int("priority"))
	}
}

func TestParseDefaults(t *testing.T) {
	flags := []Flag{
		{Name: "n", Type: "bool"},
		{Name: "msg", Type: "string"},
		{Name: "count", Type: "int"},
	}
	fv, _, err := ParseArgs(flags, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Bool("n") != false {
		t.Error("expected bool default false")
	}
	if fv.String("msg") != "" {
		t.Error("expected string default empty")
	}
	if fv.Int("count") != 0 {
		t.Error("expected int default 0")
	}
}

func TestParseMixed(t *testing.T) {
	// Simulates: grep -nC3 pattern notes/
	flags := []Flag{
		{Name: "n", Type: "bool"},
		{Name: "l", Type: "bool"},
		{Name: "c", Type: "bool"},
		{Name: "C", Type: "int"},
	}
	fv, pos, err := ParseArgs(flags, []string{"-nC3", "pattern", "notes/"})
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Bool("n") {
		t.Error("expected n=true")
	}
	if fv.Int("C") != 3 {
		t.Errorf("got C=%d, want 3", fv.Int("C"))
	}
	if len(pos) != 2 || pos[0] != "pattern" || pos[1] != "notes/" {
		t.Errorf("unexpected positional: %v", pos)
	}
}
