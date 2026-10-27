package registry

import (
	"regexp"
	"testing"
)

func TestNewHandleIs16HexCharsAndDiffers(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{16}$`)
	a, b := NewHandle(), NewHandle()
	if !re.MatchString(a) || !re.MatchString(b) || a == b {
		t.Errorf("handles %q %q", a, b)
	}
}

func TestValidHandleRefusesAnythingThatCouldLeaveTheFolder(t *testing.T) {
	good := NewHandle()
	if !ValidHandle(good) {
		t.Errorf("%q refused", good)
	}
	for _, bad := range []string{"", "../etc/passwd", "0123456789abcde", "0123456789abcdef0", "0123456789ABCDEF", "0123456789abcde/", "zzzzzzzzzzzzzzzz"} {
		if ValidHandle(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
