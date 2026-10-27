package config

import "testing"

func TestHomeIsHOMEWhenSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := Home(); got != home() || got == "" {
		t.Errorf("Home() = %q", got)
	}
}
