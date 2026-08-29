package tui

import (
	"strings"
	"testing"
)

func TestHorizBar(t *testing.T) {
	// half of width 10 -> 5 filled
	b := horizBar(0.5, 10)
	filled := strings.Count(b, string(rune(fullBlock)))
	if filled != 5 {
		t.Errorf("filled = %d, want 5 (%q)", filled, b)
	}
	// clamps above 1
	if got := horizBar(2, 4); strings.Count(got, string(rune(fullBlock))) != 4 {
		t.Errorf("over-1 not clamped: %q", got)
	}
}

func TestVBarChart(t *testing.T) {
	lines := vBarChart([]int64{0, 5, 10}, 4)
	if len(lines) != 4 {
		t.Fatalf("rows = %d, want 4", len(lines))
	}
	// every row has one char per value
	for _, l := range lines {
		if len([]rune(l)) != 3 {
			t.Errorf("row width = %d, want 3 (%q)", len([]rune(l)), l)
		}
	}
	// tallest column (value 10) should reach the top row; first column (0) empty
	topRow := []rune(lines[0])
	if topRow[2] == ' ' {
		t.Error("max column should fill the top row")
	}
	if topRow[0] != ' ' {
		t.Error("zero column should be empty at the top")
	}
}

func TestDownsample(t *testing.T) {
	// 10 values into 5 buckets, each bucket sums 2 adjacent ones
	got := downsample([]int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, 5)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	var sum int64
	for _, v := range got {
		sum += v
	}
	if sum != 10 {
		t.Errorf("sum = %d, want 10 (totals preserved)", sum)
	}
	// fewer values than width: unchanged
	if got := downsample([]int64{1, 2}, 5); len(got) != 2 {
		t.Errorf("short series changed: %v", got)
	}
}
