package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/feedinstall"
)

func TestFeedIsRefusedOnWindowsWithTheReason(t *testing.T) {
	err := feedSupported("windows")
	if err == nil || !strings.Contains(err.Error(), "Git Bash") {
		t.Errorf("%v", err)
	}
	if feedSupported("linux") != nil || feedSupported("darwin") != nil {
		t.Error("refused on unix")
	}
}

func TestAVersionedBinaryIsReplacedByTheStableLinkOnPath(t *testing.T) {
	skipWindows(t, "symlinks")
	root := t.TempDir()
	real := filepath.Join(root, "Cellar", "clalt", "1.2.3", "bin", "clalt")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte("x"), 0o755)
	link := filepath.Join(root, "bin", "clalt")
	os.MkdirAll(filepath.Dir(link), 0o755)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	look := func(string) (string, error) { return link, nil }
	// the folders of a test are temp folders, which count as unstable
	old := tempDirFn
	tempDirFn = func() string { return "/no-such-tmp" }
	t.Cleanup(func() { tempDirFn = old })

	got, note := stableBinary(real, look)

	if got != link || !strings.Contains(note, link) {
		t.Errorf("got %q note %q", got, note)
	}
	if got, note := stableBinary(link, look); got != link || note != "" {
		t.Errorf("a stable path changed: %q %q", got, note)
	}
}

func TestAVersionedBinaryWithNoMatchingLinkOnPathStaysAndSays(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "go-build123", "b001", "exe", "clalt")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte("x"), 0o755)
	other := filepath.Join(root, "other")
	os.WriteFile(other, []byte("y"), 0o755)

	got, note := stableBinary(real, func(string) (string, error) { return other, nil })

	if got != real || !strings.Contains(note, "may move") {
		t.Errorf("got %q note %q", got, note)
	}
	if _, note := stableBinary(real, func(string) (string, error) { return "", os.ErrNotExist }); !strings.Contains(note, "may move") {
		t.Errorf("note %q", note)
	}
}

func TestFeedStatusAndDoctorWarnWhenTheRecordedClaltIsGone(t *testing.T) {
	skipWindows(t, "sh commands")
	e := feedReady(t, "")
	if code := e.run("feed", "install", "--yes"); code != 0 {
		t.Fatalf("%d %s", code, e.errOut.String())
	}
	opts, _ := e.app.feedOptions()
	// point the recorded binary at a file that is not there
	st := feedinstall.Options{Settings: opts.Settings, StatePath: opts.StatePath}
	raw, _ := os.ReadFile(st.StatePath)
	os.WriteFile(st.StatePath, []byte(strings.Replace(string(raw), `"bin":"`+opts.Bin+`"`, `"bin":"/nowhere/clalt"`, 1)), 0o600)
	e.reset()

	e.run("feed", "status")
	if !strings.Contains(e.out.String(), "/nowhere/clalt") || !strings.Contains(e.out.String(), "no longer exists") {
		t.Errorf("status: %s", e.out.String())
	}
	got := defaultHomeLines(t, e)
	if c := got["feed"]; c.Status != statusWarn || !strings.Contains(c.Detail, "no longer exists") {
		t.Errorf("doctor: %+v", c)
	}
}
