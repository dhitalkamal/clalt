package main

import (
	"strings"
	"testing"
)

// the Charm v1 libraries ask the terminal for its background colour when the
// program starts (bubbletea v1 does it in an init function), and wait 5 seconds
// for an answer that some terminals never give, so every command waited. the
// dashboard is built on v2, which does not, and nothing may bring v1 back.
func TestGoModAsksForNoCharmV1Library(t *testing.T) {
	for _, line := range lines(t, "go.mod") {
		for _, banned := range []string{"github.com/charmbracelet/bubbletea ", "github.com/charmbracelet/lipgloss ", "github.com/muesli/termenv "} {
			if strings.Contains(line, banned) {
				t.Errorf("go.mod has %q: it makes every clalt command wait for a colour query on some terminals", strings.TrimSpace(line))
			}
		}
	}
}
