package tags

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid lowercase", "mytag", nil},
		{"valid with numbers", "tag123", nil},
		{"valid with hyphens", "my-tag", nil},
		{"valid mixed", "my-tag-123", nil},
		{"valid single char", "a", nil},
		{"valid max length", strings.Repeat("a", 64), nil},

		{"empty", "", ErrInvalid},
		{"too long", strings.Repeat("a", 65), ErrInvalid},
		{"uppercase", "MyTag", ErrInvalid},
		{"spaces", "my tag", ErrInvalid},
		{"underscore", "my_tag", ErrInvalid},
		{"special chars", "tag!", ErrInvalid},
		{"starts with hyphen", "-tag", nil}, // allowed per rules
		{"ends with hyphen", "tag-", nil},   // allowed per rules
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if err != tt.wantErr {
				t.Errorf("Validate(%q) = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
