package scheduler

import (
	"strings"
	"testing"
	"time"
)

func testJob() Job {
	return Job{
		Label:      "com.clalt.keepalive",
		Program:    "/usr/local/bin/clalt",
		Args:       []string{"keepalive", "--once", "--buffer", "45m"},
		Interval:   30 * time.Minute,
		StdoutPath: "/tmp/keepalive.log",
		StderrPath: "/tmp/keepalive.err",
		RunAtLoad:  true,
	}
}

func TestSystemdService(t *testing.T) {
	s := systemdService(testJob())
	for _, want := range []string{
		"[Service]",
		"Type=oneshot",
		"/usr/local/bin/clalt",
		"keepalive",
		"--buffer",
		"45m",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("service unit missing %q:\n%s", want, s)
		}
	}
}

func TestSystemdTimer(t *testing.T) {
	s := systemdTimer(testJob())
	for _, want := range []string{
		"[Timer]",
		"OnUnitActiveSec=1800", // 30m in seconds
		"Persistent=true",
		"WantedBy=timers.target",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("timer unit missing %q:\n%s", want, s)
		}
	}
}

func TestSchtasksCreateArgs(t *testing.T) {
	args := schtasksCreateArgs("com.clalt.keepalive", "C:\\clalt.exe",
		[]string{"keepalive", "--once"}, 30)
	joined := strings.Join(args, " ")
	for _, want := range []string{"/Create", "/TN", "com.clalt.keepalive", "/SC", "MINUTE", "/MO", "30", "/F"} {
		if !strings.Contains(joined, want) {
			t.Errorf("schtasks args missing %q: %v", want, args)
		}
	}
	// the /TR value must carry the program and its args
	tr := ""
	for i, a := range args {
		if a == "/TR" && i+1 < len(args) {
			tr = args[i+1]
		}
	}
	if !strings.Contains(tr, "clalt.exe") || !strings.Contains(tr, "keepalive") {
		t.Errorf("/TR command wrong: %q", tr)
	}
}

func TestIntervalMinutes(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want int
	}{
		{30 * time.Minute, 30},
		{30 * time.Second, 1}, // floor at 1
		{90 * time.Second, 2}, // rounds up from 1.5
		{0, 1},                // never zero
	}
	for _, c := range cases {
		if got := intervalMinutes(c.d); got != c.want {
			t.Errorf("intervalMinutes(%s) = %d, want %d", c.d, got, c.want)
		}
	}
}
