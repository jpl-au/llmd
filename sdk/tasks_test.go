package sdk

import "testing"

// TestTaskHasFlag covers the flag check on a task: whole names only,
// anywhere in the comma-separated set, with surrounding spaces
// tolerated and no flags at all.
func TestTaskHasFlag(t *testing.T) {
	tests := []struct {
		name  string
		flags string
		flag  string
		want  bool
	}{
		{"only flag", "human", "human", true},
		{"first of several", "blocked,human", "blocked", true},
		{"last of several", "blocked,human", "human", true},
		{"spaces around", "blocked, human", "human", true},
		{"no flags", "", "human", false},
		{"different flag", "blocked", "human", false},
		{"substring is not a flag", "unhold", "hold", false},
		{"prefix is not a flag", "human-review", "human", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := &Task{Flags: tt.flags}
			if got := tk.HasFlag(tt.flag); got != tt.want {
				t.Errorf("Task{Flags: %q}.HasFlag(%q) = %v, want %v", tt.flags, tt.flag, got, tt.want)
			}
		})
	}
}
