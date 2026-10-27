package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// plantProfile makes an old profile folder the way v0.1.0 left it: a transcript
// when history is given, and a login file when login is given.
func plantProfile(t *testing.T, name, history, login string) string {
	t.Helper()
	dir := filepath.Join(config.ProfilesDir(), name)
	mustMkdir(t, dir)
	if history != "" {
		projects := filepath.Join(dir, "projects", "-p")
		mustMkdir(t, projects)
		if err := os.WriteFile(filepath.Join(projects, "a.jsonl"), []byte(history), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if login != "" {
		if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(login), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// an old profile folder is a leftover for as long as it exists, copied or not:
// it is where v0.1.0 staged logins, and it is a second copy of the history
func TestDoctorWarnsWhileAnOldProfileFolderExists(t *testing.T) {
	for name, history := range map[string]string{"with history copied": "one", "with nothing in it": ""} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			plantProfile(t, "work", history, "")
			if code := e.run("migrate"); code != 0 {
				t.Fatalf("migrate exited %d: %s", code, e.errOut.String())
			}

			code, rep := doctor(t, e)

			rep.want(t, "leftover", "oldProfiles", "warn", "1 old profile folder")
			rep.want(t, "leftover", "oldProfiles", "warn", config.ProfilesDir())
			if code != 0 {
				t.Errorf("a leftover is a warning, exit %d", code)
			}
		})
	}
}

// a folder that is not there is the one case that is fine
func TestDoctorIsQuietWhenThereIsNoOldProfileFolder(t *testing.T) {
	e := newEnv(t)

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "ok", "no old profile folders")
}

// a file next to the folders is not a profile
func TestDoctorDoesNotCountAFileInTheProfilesFolderAsAProfile(t *testing.T) {
	e := newEnv(t)
	mustMkdir(t, config.ProfilesDir())
	if err := os.WriteFile(filepath.Join(config.ProfilesDir(), ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "ok", "no old profile folders")
}

// v0.1.0 staged plaintext logins in the profile folders. each one is named with
// its path, and the advice is to delete the files. the content is never read.
func TestDoctorNamesTheOldProfilesThatHoldAPlaintextLoginWithoutOpeningIt(t *testing.T) {
	e := newEnv(t)
	plantProfile(t, "work", "one", `{"claudeAiOauth":{"accessToken":"tok-secret-value"}}`)
	plantProfile(t, "office", "two", "")
	login := filepath.Join(config.ProfilesDir(), "work", ".credentials.json")
	if err := os.Chmod(login, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(login, 0o600) })

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", login)
	rep.want(t, "leftover", "oldProfiles", "warn", "2 old profile folders")
	rep.want(t, "leftover", "oldProfiles", "warn", "delete those files")
	if c, _ := rep.find("leftover", "oldProfiles"); strings.Contains(c.Detail, filepath.Join("office", ".credentials.json")) {
		t.Errorf("a folder without a login file is named as holding one: %q", c.Detail)
	}
	if strings.Contains(e.out.String(), "tok-secret-value") {
		t.Error("the file's content leaked")
	}
	if !exists(login) {
		t.Error("doctor deleted the file")
	}
}

// the login files are what matters most, so they are reported even when no
// history needs copying
func TestDoctorReportsAPlaintextLoginInAProfileWithNoHistory(t *testing.T) {
	e := newEnv(t)
	plantProfile(t, "work", "", "{}")

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", filepath.Join("work", ".credentials.json"))
}

// history that is not copied yet still points at migrate, next to the login
func TestDoctorKeepsThePointerAtMigrateBesideThePlaintextLogin(t *testing.T) {
	e := newEnv(t)
	plantProfile(t, "work", "one", "{}")

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", "clalt migrate")
	rep.want(t, "leftover", "oldProfiles", "warn", filepath.Join("work", ".credentials.json"))
}

// the history part needs the shared home and says so, the rest does not need it
func TestDoctorReportsAPlaintextLoginInsideARunSessionAndSaysTheHistoryWasNotChecked(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	plantProfile(t, "old", "one", "{}")
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", filepath.Join("old", ".credentials.json"))
	rep.want(t, "leftover", "oldProfiles", "warn", "clalt run")
	if c, _ := rep.find("leftover", "oldProfiles"); strings.Contains(c.Detail, "already copied") {
		t.Errorf("it says the history is copied without having looked: %q", c.Detail)
	}
}

// a profile folder that cannot be looked into may hold a login or not. saying
// nothing would read as "no login there".
func TestDoctorSaysWhenItCannotLookForALoginFileInAProfile(t *testing.T) {
	e := newEnv(t)
	dir := plantProfile(t, "work", "", "")
	if !makeUnlookable(t, dir) {
		t.Skip("cannot make a folder unreadable here")
	}

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", "could not look for a login file")
	rep.want(t, "leftover", "oldProfiles", "warn", "work")
	// and the history in it was not seen either, which is not "copied"
	rep.want(t, "leftover", "oldProfiles", "warn", "1 failed")
}

// a link to a folder, and a link that leads nowhere, are profiles as far as
// migrate is concerned. a link to a plain file is not.
func TestDoctorCountsLinksInTheProfilesFolderTheWayMigrateDoes(t *testing.T) {
	e := newEnv(t)
	elsewhere := filepath.Join(e.home, "elsewhere")
	mustMkdir(t, elsewhere)
	notes := filepath.Join(e.home, "notes.txt")
	if err := os.WriteFile(notes, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, elsewhere, filepath.Join(config.ProfilesDir(), "moved"))
	mustSymlink(t, filepath.Join(e.home, "gone"), filepath.Join(config.ProfilesDir(), "dangling"))
	mustSymlink(t, notes, filepath.Join(config.ProfilesDir(), "notes"))

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", "2 old profile folders")
}
