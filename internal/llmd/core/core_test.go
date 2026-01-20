package core

import (
	"testing"
)

func TestWriteContext_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ctx     WriteContext
		wantErr error
	}{
		{
			name: "valid context",
			ctx: WriteContext{
				Author: "test-user",
				Source: "cli",
			},
			wantErr: nil,
		},
		{
			name: "valid context with message",
			ctx: WriteContext{
				Author:  "test-user",
				Source:  "api",
				Message: "test commit message",
			},
			wantErr: nil,
		},
		{
			name:    "empty author",
			ctx:     WriteContext{Source: "cli"},
			wantErr: ErrAuthorRequired,
		},
		{
			name:    "empty source",
			ctx:     WriteContext{Author: "test-user"},
			wantErr: ErrSourceRequired,
		},
		{
			name:    "both empty",
			ctx:     WriteContext{},
			wantErr: ErrAuthorRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
