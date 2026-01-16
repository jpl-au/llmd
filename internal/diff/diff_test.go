package diff

import (
	"strings"
	"testing"
)

func TestParseVersionRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		v1      int
		v2      int
		wantErr bool
		errMsg  string
	}{
		{
			name:  "valid range",
			input: "1:3",
			v1:    1,
			v2:    3,
		},
		{
			name:  "same version",
			input: "2:2",
			v1:    2,
			v2:    2,
		},
		{
			name:  "large versions",
			input: "100:999",
			v1:    100,
			v2:    999,
		},
		{
			name:    "empty colon",
			input:   ":",
			wantErr: true,
			errMsg:  "both versions required",
		},
		{
			name:    "missing start",
			input:   ":5",
			wantErr: true,
			errMsg:  "both versions required",
		},
		{
			name:    "missing end",
			input:   "3:",
			wantErr: true,
			errMsg:  "both versions required",
		},
		{
			name:    "no colon",
			input:   "5",
			wantErr: true,
			errMsg:  "expected v1:v2",
		},
		{
			name:    "too many colons",
			input:   "1:2:3",
			wantErr: true,
			errMsg:  "expected v1:v2",
		},
		{
			name:    "non-numeric start",
			input:   "abc:5",
			wantErr: true,
			errMsg:  "invalid start version",
		},
		{
			name:    "non-numeric end",
			input:   "3:xyz",
			wantErr: true,
			errMsg:  "invalid end version",
		},
		{
			name:    "zero start",
			input:   "0:3",
			wantErr: true,
			errMsg:  "start version must be >= 1",
		},
		{
			name:    "negative start",
			input:   "-1:3",
			wantErr: true,
			errMsg:  "start version must be >= 1",
		},
		{
			name:    "zero end",
			input:   "1:0",
			wantErr: true,
			errMsg:  "end version must be >= 1",
		},
		{
			name:    "negative end",
			input:   "1:-5",
			wantErr: true,
			errMsg:  "end version must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, v2, err := ParseVersionRange(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseVersionRange(%q) = (%d, %d, nil), want error containing %q",
						tt.input, v1, v2, tt.errMsg)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseVersionRange(%q) error = %q, want containing %q",
						tt.input, err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseVersionRange(%q) = error %v, want (%d, %d)",
					tt.input, err, tt.v1, tt.v2)
				return
			}

			if v1 != tt.v1 || v2 != tt.v2 {
				t.Errorf("ParseVersionRange(%q) = (%d, %d), want (%d, %d)",
					tt.input, v1, v2, tt.v1, tt.v2)
			}
		})
	}
}
