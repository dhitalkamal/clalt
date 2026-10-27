package cli

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// v0.1.0 let two accounts have ids that differ only in case. on a file system
// that ignores case, which is what macOS and Windows have by default, they would
// be one folder and so one login.
const collidingState = `{"active":"","accounts":{` +
	`"Robin":{"email":"k@example.com","addedAt":"2026-01-01T00:00:00Z"},` +
	`"robin":{"email":"k2@example.com","addedAt":"2026-01-02T00:00:00Z"}}}`

// forceCollision makes every pair of ids that differ only in case share a folder,
// whatever file system the test runs on.
func forceCollision(t *testing.T, collide bool) {
	t.Helper()
	old := foldersCollide
	foldersCollide = func(a, b string) bool { return collide && a != b && strings.EqualFold(a, b) }
	t.Cleanup(func() { foldersCollide = old })
}

func TestLoginAndRunRefuseAccountsThatShareAFolder(t *testing.T) {
	for _, args := range [][]string{{"login", "Robin"}, {"login", "robin"}, {"run", "Robin"}, {"run", "robin"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			e := newEnv(t)
			plantState(t, collidingState)
			forceCollision(t, true)

			code := e.run(args...)

			msg := e.errOut.String()
			if code != 1 || !strings.Contains(msg, `"Robin"`) || !strings.Contains(msg, `"robin"`) || !strings.Contains(msg, "share one folder") || !strings.Contains(msg, "clalt remove") {
				t.Errorf("exit %d: %s", code, msg)
			}
			noClaudeCalls(t, e)
			noFolderOutside(t, e)
		})
	}
}

func TestDoctorReportsEveryAccountThatSharesAFolder(t *testing.T) {
	e := newEnv(t)
	plantState(t, collidingState)
	forceCollision(t, true)

	code, rep := doctor(t, e)

	rep.want(t, "Robin", "folder", "fail", "robin")
	rep.want(t, "robin", "folder", "fail", "Robin")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

func TestRemovingOneOfTwoAccountsThatShareAFolderFreesTheOther(t *testing.T) {
	e := newEnv(t)
	plantState(t, collidingState)
	forceCollision(t, true)

	// remove asks claude about the folder and signs it out, so a login is needed first
	e.claude.plantLogin(accountDir(t, "Robin"), "k@example.com")
	if code := e.run("remove", "Robin", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.claude.signInAs("k2@example.com")

	if code := e.run("login", "robin"); code != 0 {
		t.Errorf("the account that is left should work again: exit %d: %s", code, e.errOut.String())
	}
	_, rep := doctor(t, e)
	rep.want(t, "robin", "folder", "ok", "")
}

func TestAccountsThatDoNotShareAFolderAreLeftAlone(t *testing.T) {
	e := newEnv(t)
	plantState(t, collidingState)
	forceCollision(t, false) // a file system that tells the two apart
	e.claude.signInAs("k2@example.com")

	if code := e.run("login", "robin"); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	_, rep := doctor(t, e)
	rep.want(t, "robin", "folder", "ok", "")
}

func TestOnlyVisibleAccountsCanShareAFolderThatMatters(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{`+
		`"Robin":{"addedAt":"2026-01-01T00:00:00Z","hidden":true},`+
		`"robin":{"email":"k2@example.com","addedAt":"2026-01-02T00:00:00Z"}}}`)
	forceCollision(t, true)
	e.claude.signInAs("k2@example.com")

	if code := e.run("login", "robin"); code != 0 {
		t.Errorf("a removed account holds no login to clash with: exit %d: %s", code, e.errOut.String())
	}
}

func TestFoldersCollideLooksAtTheRealFolders(t *testing.T) {
	newEnv(t)
	if foldersCollide("robin", "robin") || foldersCollide("a", "b") {
		t.Error("ids that are the same, or differ in more than case, never collide")
	}
	// neither folder exists yet: what the platform usually does
	want := runtime.GOOS == "darwin" || runtime.GOOS == "windows"
	if got := foldersCollide("Robin", "robin"); got != want {
		t.Errorf("no folders yet: foldersCollide = %v, want %v on %s", got, want, runtime.GOOS)
	}

	// one exists: a file system that ignores case would show both, so this one does not
	if err := os.MkdirAll(accountDir(t, "Robin"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(accountDir(t, "robin"))
	ignoresCase := err == nil
	if got := foldersCollide("Robin", "robin"); got != ignoresCase {
		t.Errorf("one folder: foldersCollide = %v, but the file system ignores case = %v", got, ignoresCase)
	}
	if !ignoresCase {
		if err := os.MkdirAll(accountDir(t, "robin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if foldersCollide("Robin", "robin") {
			t.Error("two different folders are not a collision")
		}
	}
}

func TestFoldersCollideNeverBuildsAPathFromAnUnsafeId(t *testing.T) {
	newEnv(t)
	for _, pair := range [][2]string{{"CON", "con"}, {"X/y", "x/Y"}, {"a\x00b", "A\x00B"}} {
		if foldersCollide(pair[0], pair[1]) {
			t.Errorf("%q is not a safe folder name, so it must not be looked up on disk", pair[0])
		}
	}
}

// the kelvin sign folds to k. it is not a valid label, but as an old id it is a
// safe folder name, and on a file system that ignores case it is k's folder.
func TestFoldersCollideSeesAnOldIdThatOnlyFoldsToAnother(t *testing.T) {
	newEnv(t)
	want := runtime.GOOS == "darwin" || runtime.GOOS == "windows"
	if got := foldersCollide("k", "\u212a"); got != want {
		t.Errorf("foldersCollide = %v, want %v on %s", got, want, runtime.GOOS)
	}
}

// three ids that differ only in case: the removed one does not count, the other
// two still clash with each other and are named, and it is not named.
func TestAGroupOfThreeIdsOnlyCountsTheVisibleOnes(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{`+
		`"Robin":{"email":"a@example.com","addedAt":"2026-01-01T00:00:00Z"},`+
		`"robin":{"addedAt":"2026-01-02T00:00:00Z","hidden":true},`+
		`"ROBIN":{"email":"c@example.com","addedAt":"2026-01-03T00:00:00Z"}}}`)
	forceCollision(t, true)

	code := e.run("run", "Robin")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, `"ROBIN"`) || strings.Contains(msg, `"robin"`) {
		t.Errorf("exit %d: %s", code, msg)
	}
}

func TestAnAccountOutsideTheCollidingGroupIsNotRefused(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{`+
		`"Robin":{"email":"a@example.com","addedAt":"2026-01-01T00:00:00Z"},`+
		`"robin":{"email":"b@example.com","addedAt":"2026-01-02T00:00:00Z"},`+
		`"work":{"email":"w@example.com","addedAt":"2026-01-03T00:00:00Z"}}}`)
	forceCollision(t, true)
	e.claude.signInAs("w@example.com")

	if code := e.run("login", "work"); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	_, rep := doctor(t, e)
	rep.want(t, "work", "folder", "ok", "")
}
