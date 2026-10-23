package crew

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTheCrewFolderIsInsideClaltsOwnFolder(t *testing.T) {
	home := isolate(t)
	want := filepath.Join(home, ".config", "clalt", "crew")
	if got := Root(); got != want {
		t.Errorf("Root() = %q, want %q", got, want)
	}
	if got := LockPath(); got != filepath.Join(want, ".lock") {
		t.Errorf("LockPath() = %q", got)
	}
}

func TestAnAccountsPathsAreAllUnderItsOwnFolder(t *testing.T) {
	isolate(t)
	p, err := PathsFor("work")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(Root(), "work")
	for name, got := range map[string]string{
		"Dir": p.Dir, "Record": p.Record, "Log": p.Log, "PrevLog": p.PrevLog, "Home": p.Home, "LocalConfig": p.LocalConfig,
	} {
		if got != dir && !strings.HasPrefix(got, dir+string(filepath.Separator)) {
			t.Errorf("%s = %q, not inside %q", name, got, dir)
		}
	}
	if filepath.Base(p.Record) != "gateway.json" || filepath.Base(p.Log) != "gateway.out" || filepath.Base(p.PrevLog) != "gateway.out.prev" {
		t.Errorf("unexpected file names: %+v", p)
	}
	if p.LocalConfig != filepath.Join(dir, "home", "config.local.json") {
		t.Errorf("LocalConfig = %q", p.LocalConfig)
	}
	if p.Home != filepath.Join(dir, "home") || p.KiroHome != filepath.Join(dir, "home", "kiro") || p.Workspace != filepath.Join(dir, "home", "workspace") {
		t.Errorf("unexpected data folders: %+v", p)
	}
}

func TestAnIdThatIsNotOneFolderNameIsRefused(t *testing.T) {
	isolate(t)
	for _, id := range []string{"", ".", "..", "a/b", `a\b`, "../escape", "nul\x00byte", "con"} {
		if p, err := PathsFor(id); err == nil {
			t.Errorf("PathsFor(%q) = %+v, want an error", id, p)
		}
	}
}

func TestWithNoHomeThePathsAreRefusedAndNotMadeRelative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows falls back to USERPROFILE, so there is always a home")
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if p, err := PathsFor("work"); err == nil {
		t.Errorf("PathsFor with no home = %+v, want an error", p)
	}
}
