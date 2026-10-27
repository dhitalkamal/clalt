package dashboard_test

import (
	"strings"
	"testing"
)

func footer(out string) string {
	lines := strings.Split(out, "\n")
	return lines[len(lines)-1]
}

func TestRenameOnDemoDataOnlySaysWhatItWouldDo(t *testing.T) {
	m := sized(config{ascii: true, noColor: true}, 100, 30)
	m, _ = press(m, "r", "backspace", "backspace", "backspace", "backspace", "backspace")
	m = typeText(m, "work")
	m, cmd := press(m, "enter")
	out := screen(settle(m, cmd))
	if got := footer(out); got != "  Demo: would rename robin to work." {
		t.Errorf("the demo says what it would do: %q", got)
	}
	if !strings.Contains(out, "> robin") {
		t.Errorf("nothing changes in the demo:\n%s", out)
	}
}

func TestRenameOnDemoDataRefusesANameAnotherAccountHas(t *testing.T) {
	m := sized(config{ascii: true, noColor: true}, 100, 30)
	m, _ = press(m, "r", "backspace", "backspace", "backspace", "backspace", "backspace")
	m = typeText(m, "MORGAN")
	m, _ = press(m, "enter")
	if got := footer(screen(m)); !strings.HasSuffix(got, "  Another account has that name.") {
		t.Errorf("the demo accounts take part in the check: %q", got)
	}
}

func TestSignOutOnDemoDataAsksThenOnlySaysWhatItWouldDo(t *testing.T) {
	m := sized(config{ascii: true, noColor: true}, 100, 30)
	asked, _ := press(m, "d")
	if got := footer(screen(asked)); got != "  y signs out robin and hides it. Chat history is kept. Any other key cancels." {
		t.Errorf("the question comes first: %q", got)
	}
	done, cmd := press(asked, "y")
	out := screen(settle(done, cmd))
	if got := footer(out); got != "  Demo: would sign out robin (history kept)." {
		t.Errorf("the demo says what it would do: %q", got)
	}
	if !strings.Contains(out, "> robin     26%") {
		t.Errorf("nothing changes in the demo:\n%s", out)
	}
}

func TestSignOutOfTheSignedOutDemoAccountIsOnlyAMessage(t *testing.T) {
	m := sized(config{ascii: true, noColor: true}, 100, 30)
	m, _ = press(m, "j", "j", "j", "j", "d")
	if got := footer(screen(m)); got != "  harriet is already signed out." {
		t.Errorf("no question for harriet: %q", got)
	}
}
