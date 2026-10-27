package entities_test

import (
	"testing"
)

// TestWrite covers writing an entity with and without a relation. Every
// case checks the returned entity echoes its inputs and origin and has a
// generated key.
func TestWrite(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		value     string
		relation  string
	}{
		{"no relation", "test:item", `{"name":"foo"}`, ""},
		{"with relation", "test:item", `{"name":"bar"}`, "docs/readme"},
		{"other namespace without relation", "config:setting", `{"theme":"dark"}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			opts := testWriteOpts()
			opts.Relation = tt.relation

			got, err := s.Entities.Write(t.Context(), tt.namespace, tt.value, opts)
			if err != nil {
				t.Fatalf("Write(%q, %q) error = %v", tt.namespace, tt.value, err)
			}
			if got.Namespace != tt.namespace {
				t.Errorf("Write(%q, %q) Namespace = %q, want %q", tt.namespace, tt.value, got.Namespace, tt.namespace)
			}
			if got.Value != tt.value {
				t.Errorf("Write(%q, %q) Value = %q, want %q", tt.namespace, tt.value, got.Value, tt.value)
			}
			if got.Relation != tt.relation {
				t.Errorf("Write(%q, %q) Relation = %q, want %q", tt.namespace, tt.value, got.Relation, tt.relation)
			}
			if len(got.Key) != 10 {
				t.Errorf("Write(%q, %q) Key length = %d, want 9", tt.namespace, tt.value, len(got.Key))
			}
			if got.Author != "test" {
				t.Errorf("Write(%q, %q) Author = %q, want %q", tt.namespace, tt.value, got.Author, "test")
			}
			if got.Source != "cli" {
				t.Errorf("Write(%q, %q) Source = %q, want %q", tt.namespace, tt.value, got.Source, "cli")
			}
		})
	}
}
