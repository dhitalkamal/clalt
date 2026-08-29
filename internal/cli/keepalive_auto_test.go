package cli

import "testing"

func TestShouldAutoInstallKeepalive(t *testing.T) {
	cases := []struct {
		name         string
		envDisabled  bool
		userDisabled bool
		alreadyMark  bool
		numAccounts  int
		want         bool
	}{
		{"fresh with accounts", false, false, false, 2, true},
		{"no accounts yet", false, false, false, 0, false},
		{"already marked", false, false, true, 2, false},
		{"user disabled", false, true, false, 2, false},
		{"env disabled", true, false, false, 2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldAutoInstallKeepalive(c.envDisabled, c.userDisabled, c.alreadyMark, c.numAccounts)
			if got != c.want {
				t.Errorf("shouldAutoInstallKeepalive(env=%v user=%v mark=%v n=%d) = %v, want %v",
					c.envDisabled, c.userDisabled, c.alreadyMark, c.numAccounts, got, c.want)
			}
		})
	}
}
