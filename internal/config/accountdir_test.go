package config

import (
	"path/filepath"
	"testing"
)

func TestAccountDirForAcceptsOneSafeElement(t *testing.T) {
	// a temp dir is absolute on every system, "/xdg" is not on windows
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	// the last ones fail account.ValidateName but are legacy ids that must keep working
	for _, id := range []string{"robin", "Robin", "robin-2", "a.b", "x_y", "com10", "nulled", "weird name!", "caf\u00e9"} {
		got, err := AccountDirFor(id)
		if err != nil {
			t.Errorf("AccountDirFor(%q): %v", id, err)
			continue
		}
		if want := filepath.Join(xdg, "clalt", "accounts", id); got != want {
			t.Errorf("AccountDirFor(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestAccountDirForRefusesAnythingThatIsNotOneLocalName(t *testing.T) {
	// an absolute clalt dir, so it is the id that gets refused
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	bad := []string{
		"", ".", "..", "a/b", `a\b`, "../x", "..\\x", "/abs", "a\x00b",
		"con", "CON", "Con.txt", "nul", "aux.tar.gz", "prn", "com1", "COM9", "lpt1", "LPT9.log",
	}
	for _, id := range bad {
		if got, err := AccountDirFor(id); err == nil {
			t.Errorf("AccountDirFor(%q) = %q, want an error", id, got)
		}
	}
}

func TestAccountDirForNeedsAnAbsoluteClaltDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	// windows asks for this one when HOME is empty
	t.Setenv("USERPROFILE", "")
	if got, err := AccountDirFor("robin"); err == nil {
		t.Errorf("AccountDirFor = %q with no HOME, want an error", got)
	}
}

// the id is judged on its own: a machine without a home has no folder for any
// id, and that is not a reason to call an id unsafe.
func TestCheckAccountIDLooksAtTheIdAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	for _, id := range []string{"robin", "robin-2", "weird name!", "caf\u00e9"} {
		if err := CheckAccountID(id); err != nil {
			t.Errorf("CheckAccountID(%q): %v", id, err)
		}
	}
	for _, id := range []string{"", "..", "a/b", `a\b`, "../../escape", "a\x00b", "con", "Nul.txt"} {
		if err := CheckAccountID(id); err == nil {
			t.Errorf("CheckAccountID(%q) accepted an id that cannot be a folder name", id)
		}
	}
}
