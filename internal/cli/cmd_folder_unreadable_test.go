package cli

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// makeUnlookable takes away the right to look inside dir, so that asking about
// one of its entries fails with something other than "not there". it reports
// false where that cannot work: windows has no such bits and root ignores them.
func makeUnlookable(t *testing.T, dir string) bool {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return false
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	return true
}

// a folder that cannot be looked at is not a folder without a login. only "not
// there" means absent, and everything else is unknown: the views say so, and
// nothing that signs out or removes goes ahead on a guess.
func signedInAccountWithAnUnlookableFolder(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	if !makeUnlookable(t, config.AccountsDir()) {
		t.Skip("no permission bits to take away")
	}
	e.reset()
	return e
}

func TestListSaysUnknownForAFolderThatCannotBeLookedAt(t *testing.T) {
	e := signedInAccountWithAnUnlookableFolder(t)

	row := listJSON(t, e)[0]

	if row["signedIn"] != nil || !strings.Contains(row["reason"].(string), "permission denied") {
		t.Errorf("row = %v: want unknown, with the reason", row)
	}
	if !strings.Contains(e.out.String(), "work") {
		t.Errorf("output:\n%s", e.out.String())
	}
}

func TestLogoutAndRemoveStopWhenTheFolderCannotBeLookedAt(t *testing.T) {
	for _, args := range [][]string{{"logout", "work"}, {"remove", "work", "--force"}} {
		t.Run(args[0], func(t *testing.T) {
			e := signedInAccountWithAnUnlookableFolder(t)
			before := e.account("work").Meta

			code := e.run(args...)

			msg := e.errOut.String()
			if code != 1 || !strings.Contains(msg, "permission denied") || !strings.Contains(msg, "nothing was changed") {
				t.Errorf("exit %d, stderr %q", code, msg)
			}
			if strings.Contains(e.out.String(), "signed out") || strings.Contains(e.out.String(), "removed work") {
				t.Errorf("it claimed something that did not happen:\n%s", e.out.String())
			}
			if after := e.account("work").Meta; after != before {
				t.Errorf("the record changed although nothing was done: %+v then %+v", before, after)
			}
			if calls := e.claude.calls("logout"); len(calls) != 0 {
				t.Errorf("claude was asked to sign out: %v", calls)
			}
		})
	}
}

func TestDoctorDoesNotCallAFolderThatCannotBeLookedAtMissing(t *testing.T) {
	e := signedInAccountWithAnUnlookableFolder(t)

	code, rep := doctor(t, e)

	folder, ok := rep.find("work", "folder")
	if !ok {
		t.Fatalf("no folder check: %+v", rep.Checks)
	}
	if folder.Status != "warn" || strings.Contains(folder.Detail, "missing") || !strings.Contains(folder.Detail, "permission denied") {
		t.Errorf("folder = %s %q: it is there, it cannot be looked at", folder.Status, folder.Detail)
	}
	rep.want(t, "work", "signedIn", "warn", "permission denied")
	if code != 0 {
		t.Errorf("exit %d: not being able to look is a warning", code)
	}
}
