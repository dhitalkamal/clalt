package layout

import "testing"

func TestValidAccountTakesTheReservedCopilotIdAndNoOtherTilde(t *testing.T) {
	for _, ok := range []string{"work", "a.b_c-1", "~copilot"} {
		if !ValidAccount(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "~other", "~copilot2", "~", "a~", "~~copilot", "~Copilot"} {
		if ValidAccount(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestACabinForTheCopilotTeamValidates(t *testing.T) {
	l := valid2()
	l.Pavilions[1].Account = "~copilot"
	if err := Validate(l, nil); err != nil {
		t.Errorf("copilot cabin: %v", err)
	}
	l.Pavilions[1].Account = "~copilot2"
	if err := Validate(l, nil); err == nil {
		t.Error("~copilot2 accepted")
	}
}
