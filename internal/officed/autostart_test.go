package officed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeLaunchctl struct {
	calls [][]string
	fail  map[string]error
}

func (f *fakeLaunchctl) run(args []string) ([]byte, error) {
	f.calls = append(f.calls, args)
	return nil, f.fail[args[0]]
}

func newAutostart(t *testing.T) (Autostart, *fakeLaunchctl) {
	f := &fakeLaunchctl{fail: map[string]error{}}
	home := t.TempDir()
	return Autostart{Home: home, UID: 501, Exe: "/opt/bin/clalt", LogPath: filepath.Join(home, "office.log"), GOOS: "darwin", Run: f.run}, f
}

func TestPlistContent(t *testing.T) {
	a, _ := newAutostart(t)
	p := string(a.Plist())
	for _, want := range []string{
		"<string>com.dhitalkamal.clalt.office</string>", "<string>/opt/bin/clalt</string>",
		"<string>office</string>", "<string>serve</string>", "<key>RunAtLoad</key>\n\t<true/>",
		"<key>KeepAlive</key>\n\t<false/>", "<key>StandardOutPath</key>", "<key>StandardErrorPath</key>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist lacks %q:\n%s", want, p)
		}
	}
	if strings.Contains(strings.ToLower(p), "token") {
		t.Errorf("plist mentions a token:\n%s", p)
	}
}

func TestPlistEscapesThePath(t *testing.T) {
	a, _ := newAutostart(t)
	a.Exe = "/Users/a&b/<x>/clalt"
	if p := string(a.Plist()); !strings.Contains(p, "/Users/a&amp;b/&lt;x&gt;/clalt") {
		t.Errorf("not escaped:\n%s", p)
	}
}

func TestEnableWritesThePlistAndBootstraps(t *testing.T) {
	a, f := newAutostart(t)
	if err := a.Enable(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.Home, "Library", "LaunchAgents", "com.dhitalkamal.clalt.office.plist")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	last := f.calls[len(f.calls)-1]
	if strings.Join(last, " ") != "bootstrap gui/501 "+path {
		t.Errorf("calls %v", f.calls)
	}
	if !a.Enabled() {
		t.Error("not enabled")
	}
}

func TestEnableTwiceBootsOutFirst(t *testing.T) {
	a, f := newAutostart(t)
	a.Enable()
	f.calls = nil
	if err := a.Enable(); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || f.calls[0][0] != "bootout" || f.calls[1][0] != "bootstrap" {
		t.Errorf("calls %v", f.calls)
	}
}

func TestDisableBootsOutAndRemovesTheFile(t *testing.T) {
	a, f := newAutostart(t)
	a.Enable()
	f.calls = nil
	if err := a.Disable(true); err != nil {
		t.Fatal(err)
	}
	if a.Enabled() || len(f.calls) != 1 || f.calls[0][0] != "bootout" || f.calls[0][1] != "gui/501" {
		t.Errorf("enabled %v calls %v", a.Enabled(), f.calls)
	}
}

func TestDisableWithoutUnloadOnlyRemovesTheFile(t *testing.T) {
	a, f := newAutostart(t)
	a.Enable()
	f.calls = nil
	a.Disable(false)
	if a.Enabled() || len(f.calls) != 0 {
		t.Errorf("enabled %v calls %v", a.Enabled(), f.calls)
	}
}

func TestAutostartIsMacOnly(t *testing.T) {
	a, f := newAutostart(t)
	a.GOOS = "linux"
	if a.Supported() || a.Enable() == nil || len(f.calls) != 0 {
		t.Error("linux must refuse and call nothing")
	}
	if _, err := os.Stat(filepath.Join(a.Home, "Library")); err == nil {
		t.Error("linux wrote files")
	}
}

func TestEnableFailureLeavesNoPlist(t *testing.T) {
	a, f := newAutostart(t)
	f.fail["bootstrap"] = os.ErrPermission
	if err := a.Enable(); err == nil || a.Enabled() {
		t.Errorf("err %v enabled %v", err, a.Enabled())
	}
}
