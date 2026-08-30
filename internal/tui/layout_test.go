package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderRobustAcrossSizes(t *testing.T) {
	d := sampleData()
	for w := 40; w <= 220; w += 7 {
		for h := 10; h <= 62; h += 5 {
			out := Render(d, w, h) // must not panic
			// above the minimum, every visual line must fit the width
			if w >= minWidth && h >= minHeight {
				for _, ln := range strings.Split(out, "\n") {
					if lipgloss.Width(ln) > w {
						t.Fatalf("line wider than %d at %dx%d: %q (%d)", w, w, h, ln, lipgloss.Width(ln))
					}
				}
			}
		}
	}
}

func TestRenderEmptyNoPanic(t *testing.T) {
	_ = Render(Data{Version: "v0"}, 120, 40)
}
