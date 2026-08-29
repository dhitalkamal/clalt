package launchd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistContainsAgentFields(t *testing.T) {
	a := Agent{
		Label:      "com.clalt.keepalive",
		Program:    "/usr/local/bin/clalt",
		Args:       []string{"keepalive", "--once", "--buffer", "45m"},
		Interval:   1800,
		StdoutPath: "/tmp/clalt/keepalive.log",
		StderrPath: "/tmp/clalt/keepalive.err",
		RunAtLoad:  true,
	}
	got := a.Plist()

	// the program is the first ProgramArguments entry, followed by each arg
	for _, want := range []string{
		"com.clalt.keepalive",
		"<string>/usr/local/bin/clalt</string>",
		"<string>keepalive</string>",
		"<string>--once</string>",
		"<string>--buffer</string>",
		"<string>45m</string>",
		"<key>StartInterval</key>",
		"<integer>1800</integer>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"/tmp/clalt/keepalive.log",
		"/tmp/clalt/keepalive.err",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
}

func TestPlistProgramComesBeforeArgs(t *testing.T) {
	a := Agent{
		Label:    "com.clalt.keepalive",
		Program:  "/bin/clalt",
		Args:     []string{"keepalive", "--once"},
		Interval: 60,
	}
	got := a.Plist()
	prog := strings.Index(got, "<string>/bin/clalt</string>")
	arg := strings.Index(got, "<string>keepalive</string>")
	if prog < 0 || arg < 0 || prog > arg {
		t.Errorf("program must precede args: prog=%d arg=%d\n%s", prog, arg, got)
	}
}

func TestPlistOmitsRunAtLoadWhenFalse(t *testing.T) {
	a := Agent{Label: "x", Program: "/bin/clalt", Interval: 60}
	if strings.Contains(a.Plist(), "<key>RunAtLoad</key>") {
		t.Error("RunAtLoad key should be absent when false")
	}
}

func TestPlistPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	got := PlistPath("com.clalt.keepalive")
	want := filepath.Join("/home/tester", "Library", "LaunchAgents", "com.clalt.keepalive.plist")
	if got != want {
		t.Errorf("PlistPath = %q, want %q", got, want)
	}
}
