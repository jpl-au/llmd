package llmd

import "testing"

// TestJSONField covers the value extraction used for tag and link
// details: the field when present, otherwise the raw value unchanged.
func TestJSONField(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		field string
		want  string
	}{
		{"tag field", `{"tag":"urgent"}`, "tag", "urgent"},
		{"link field", `{"to":"docs/b","from":"docs/a"}`, "to", "docs/b"},
		{"missing field", `{"from":"docs/a"}`, "to", `{"from":"docs/a"}`},
		{"empty object", `{}`, "tag", `{}`},
		{"not json", "urgent", "tag", "urgent"},
		{"empty string", "", "tag", ""},
		{"non-string value", `{"tag":1}`, "tag", `{"tag":1}`},
		{"json array", `["urgent"]`, "tag", `["urgent"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonField(tt.raw, tt.field); got != tt.want {
				t.Errorf("jsonField(%q, %q) = %q, want %q", tt.raw, tt.field, got, tt.want)
			}
		})
	}
}
