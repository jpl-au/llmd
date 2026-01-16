package edit

import (
	"errors"
	"testing"
)

func TestParseLineRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		start   int
		end     int
		wantErr bool
		errMsg  string
	}{
		{
			name:  "valid range",
			input: "5:10",
			start: 5,
			end:   10,
		},
		{
			name:  "single line",
			input: "1:1",
			start: 1,
			end:   1,
		},
		{
			name:    "empty colon",
			input:   ":",
			wantErr: true,
			errMsg:  "at least start or end line required",
		},
		{
			name:  "open-ended start",
			input: ":10",
			start: 0,
			end:   10,
		},
		{
			name:  "open-ended end",
			input: "5:",
			start: 5,
			end:   0,
		},
		{
			name:    "no colon",
			input:   "5",
			wantErr: true,
			errMsg:  "expected start:end",
		},
		{
			name:    "too many colons",
			input:   "1:2:3",
			wantErr: true,
			errMsg:  "expected start:end",
		},
		{
			name:    "non-numeric start",
			input:   "abc:10",
			wantErr: true,
			errMsg:  "invalid start line",
		},
		{
			name:    "non-numeric end",
			input:   "5:xyz",
			wantErr: true,
			errMsg:  "invalid end line",
		},
		{
			name:    "zero start",
			input:   "0:10",
			wantErr: true,
			errMsg:  "start line must be >= 1",
		},
		{
			name:    "negative start",
			input:   "-1:10",
			wantErr: true,
			errMsg:  "start line must be >= 1",
		},
		{
			name:    "zero end",
			input:   "1:0",
			wantErr: true,
			errMsg:  "end line must be >= 1",
		},
		{
			name:    "negative end",
			input:   "1:-5",
			wantErr: true,
			errMsg:  "end line must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := ParseLineRange(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseLineRange(%q) = (%d, %d, nil), want error containing %q",
						tt.input, start, end, tt.errMsg)
					return
				}
				if !errors.Is(err, ErrInvalidLineRange) {
					t.Errorf("ParseLineRange(%q) error = %v, want ErrInvalidLineRange", tt.input, err)
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseLineRange(%q) error = %q, want containing %q",
						tt.input, err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseLineRange(%q) = error %v, want (%d, %d)",
					tt.input, err, tt.start, tt.end)
				return
			}

			if start != tt.start || end != tt.end {
				t.Errorf("ParseLineRange(%q) = (%d, %d), want (%d, %d)",
					tt.input, start, end, tt.start, tt.end)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
