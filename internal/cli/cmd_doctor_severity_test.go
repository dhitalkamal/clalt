package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// doctor fails for what is really broken. what the user has to do something
// about is a warning, so that a plain logout, or the first look after upgrading
// from v0.1.0, does not read as an error.
func TestDoctorAfterALogoutExitsZeroWithWarnings(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	if code := e.run("logout", "work"); code != 0 {
		t.Fatalf("logout exited %d: %s", code, e.errOut.String())
	}

	code, rep := doctor(t, e)

	if code != 0 || rep.Failed != 0 || rep.Warned == 0 {
		t.Errorf("exit %d, %d failed, %d warned: %+v", code, rep.Failed, rep.Warned, rep.Checks)
	}
	rep.want(t, "work", "signedIn", "warn", "clalt login work")
	rep.want(t, "home", "signedIn", "ok", "")
}

func TestDoctorRightAfterUpgradingFromV010ExitsZeroWithWarnings(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"work","accounts":{`+
		`"work":{"email":"w@example.com","plan":"max","addedAt":"2026-08-01T10:00:00Z"},`+
		`"home":{"email":"h@example.com","plan":"pro","addedAt":"2026-08-02T10:00:00Z"}}}`)
	// what v0.1.0 left behind
	if err := os.MkdirAll(config.VaultDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.VaultKeyPath(), []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(config.ProfilesDir(), "work", "projects", "-p", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	if code != 0 || rep.Failed != 0 {
		t.Fatalf("exit %d, %d failed: %+v", code, rep.Failed, rep.Checks)
	}
	for _, label := range []string{"work", "home"} {
		rep.want(t, label, "folder", "warn", "clalt login "+label)
		rep.want(t, label, "signedIn", "warn", "clalt login "+label)
	}
	rep.want(t, "leftover", "oldVault", "warn", "vault")
	rep.want(t, "leftover", "oldProfiles", "warn", "clalt migrate")
	noClaudeCalls(t, e)
}

// the only thing about an account that fails doctor is a login of somebody
// else's: running it would use their account.
func TestDoctorStillFailsForADifferentIdentityAndForAProjectsFolderThatIsNotLinked(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.claude.plantLogin(e.folder("work"), "other@example.com")

	code, rep := doctor(t, e)

	rep.want(t, "work", "identity", "fail", "w@example.com")
	if code != 1 || rep.Failed != 1 {
		t.Errorf("exit %d, %d failed, want one failure: %+v", code, rep.Failed, rep.Checks)
	}
}

// the refusal for a login that something outside the account overrides says
// where to look, since signing in again would not change what claude reports.
func TestRunSaysWhereToLookWhenSomethingOverridesTheLogin(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.set("API_KEY_HELPER", "1")

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || strings.Contains(msg, "sign in with") || !strings.Contains(msg, "clalt doctor") {
		t.Errorf("exit %d: %s", code, msg)
	}
}

func TestRunStillSaysToSignInWhenTheFolderIsSignedOut(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signOut(e.folder("work"))

	code := e.run("run", "work")

	if msg := e.errOut.String(); code != 1 || !strings.Contains(msg, "sign in with: clalt login work") {
		t.Errorf("exit %d: %s", code, msg)
	}
}
