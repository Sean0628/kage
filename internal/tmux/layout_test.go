package tmux

import (
	"testing"
)

func TestCalcRelativeSplitSizes(t *testing.T) {
	tests := []struct {
		name   string
		panes  []int
		expect []int
	}{
		{
			name:   "60/20/20 default layout",
			panes:  []int{60, 20, 20},
			expect: []int{40, 50},
		},
		{
			name:   "50/25/25",
			panes:  []int{50, 25, 25},
			expect: []int{50, 50},
		},
		{
			name:   "two panes 70/30",
			panes:  []int{70, 30},
			expect: []int{30},
		},
		{
			name:   "single pane",
			panes:  []int{100},
			expect: nil,
		},
		{
			name:   "empty",
			panes:  []int{},
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcRelativeSplitSizes(tt.panes)
			if len(got) != len(tt.expect) {
				t.Fatalf("expected %d splits, got %d: %v", len(tt.expect), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("split[%d]: expected %d, got %d", i, tt.expect[i], got[i])
				}
			}
		})
	}
}

func TestParseSizePercent(t *testing.T) {
	tests := []struct {
		input  string
		expect int
	}{
		{"60%", 60},
		{"20%", 20},
		{"100%", 100},
		{"  50% ", 50},
	}

	for _, tt := range tests {
		got := ParseSizePercent(tt.input)
		if got != tt.expect {
			t.Errorf("ParseSizePercent(%q) = %d, want %d", tt.input, got, tt.expect)
		}
	}
}
