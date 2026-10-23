package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/profile"
)

// unlookableAccount is an account folder whose projects entry cannot be asked
// about: the folder is there, but the one above it has no right to look inside.
func unlookableAccount(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	account := filepath.Join(dir, "account")
	mustMkdir(t, account)
	if !makeUnlookable(t, dir) {
		t.Skip("no permission bits to take away")
	}
	return account
}

// an entry that cannot be looked at is not an entry that is not there, and the
// reason is kept for whoever has to say something about it
func TestHistoryOfKeepsTheReasonWhenTheEntryCannotBeLookedAt(t *testing.T) {
	newEnv(t)
	account := unlookableAccount(t)

	state, err := historyOf(account)

	if state != historyUnknown || err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("historyOf = %v, %v: want unknown, with the reason", state, err)
	}
}

// login's warning and run's refusal say why, and do not blame an unmounted
// volume for a permission
func TestWhatIsSaidAboutAnUnlookableHistoryNamesThePermission(t *testing.T) {
	e := newEnv(t)
	account := unlookableAccount(t)

	e.app.warnHistory("work", account)
	problem := projectsProblem(profile.Report{}, account)

	for name, got := range map[string]string{"login's warning": e.errOut.String(), "run's refusal": problem} {
		if !strings.Contains(got, "permission denied") || strings.Contains(got, "unmounted") {
			t.Errorf("%s: %q", name, got)
		}
	}
	if !strings.Contains(e.errOut.String(), "'clalt run work' will refuse") {
		t.Errorf("login's warning does not say what run will do: %q", e.errOut.String())
	}
}
