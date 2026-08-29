package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// systemdSched is the Linux backend: a user-level oneshot service driven by a
// timer, installed under the user's systemd config with no root required.
type systemdSched struct{}

func (systemdSched) Name() string { return "systemd" }

func (systemdSched) Install(j Job) error {
	unitDir := systemdUnitDir()
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}
	svc := filepath.Join(unitDir, j.Label+".service")
	tmr := filepath.Join(unitDir, j.Label+".timer")
	if err := os.WriteFile(svc, []byte(systemdService(j)), 0o644); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}
	if err := os.WriteFile(tmr, []byte(systemdTimer(j)), 0o644); err != nil {
		return fmt.Errorf("write timer unit: %w", err)
	}
	_ = runCmd("systemctl", "--user", "daemon-reload")
	if err := runCmd("systemctl", "--user", "enable", "--now", j.Label+".timer"); err != nil {
		return fmt.Errorf("enable timer: %w", err)
	}
	if j.RunAtLoad {
		_ = runCmd("systemctl", "--user", "start", j.Label+".service")
	}
	return nil
}

func (systemdSched) Uninstall(label string) error {
	_ = runCmd("systemctl", "--user", "disable", "--now", label+".timer")
	dir := systemdUnitDir()
	for _, p := range []string{filepath.Join(dir, label+".timer"), filepath.Join(dir, label+".service")} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	_ = runCmd("systemctl", "--user", "daemon-reload")
	return nil
}

func (systemdSched) Status(label string) (bool, error) {
	// is-enabled exits 0 for an enabled timer; a non-zero exit just means "not
	// enabled", which is a valid answer rather than an error.
	err := exec.Command("systemctl", "--user", "is-enabled", label+".timer").Run()
	return err == nil, nil
}

func (systemdSched) Location(label string) string {
	return filepath.Join(systemdUnitDir(), label+".timer")
}

func (systemdSched) Plan(j Job) string {
	dir := systemdUnitDir()
	svc := filepath.Join(dir, j.Label+".service")
	tmr := filepath.Join(dir, j.Label+".timer")
	return fmt.Sprintf("systemd: write %s\n\n%s\nwrite %s\n\n%s\nthen: systemctl --user daemon-reload && systemctl --user enable --now %s.timer",
		svc, systemdService(j), tmr, systemdTimer(j), j.Label)
}

// systemdService renders the oneshot service unit that runs the job once.
func systemdService(j Job) string {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	fmt.Fprintf(&b, "Description=clalt background job (%s)\n\n", j.Label)
	b.WriteString("[Service]\n")
	b.WriteString("Type=oneshot\n")
	fmt.Fprintf(&b, "ExecStart=%s\n", execLine(j.Program, j.Args))
	if j.StdoutPath != "" {
		fmt.Fprintf(&b, "StandardOutput=append:%s\n", j.StdoutPath)
	}
	if j.StderrPath != "" {
		fmt.Fprintf(&b, "StandardError=append:%s\n", j.StderrPath)
	}
	return b.String()
}

// systemdTimer renders the timer unit that fires the service periodically.
func systemdTimer(j Job) string {
	secs := int(j.Interval.Seconds())
	if secs < 1 {
		secs = 1
	}
	var b strings.Builder
	b.WriteString("[Unit]\n")
	fmt.Fprintf(&b, "Description=clalt background timer (%s)\n\n", j.Label)
	b.WriteString("[Timer]\n")
	// fire the first time shortly after login, then every interval; Persistent
	// catches up a run missed while logged out.
	fmt.Fprintf(&b, "OnBootSec=%d\n", secs)
	fmt.Fprintf(&b, "OnUnitActiveSec=%d\n", secs)
	b.WriteString("Persistent=true\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=timers.target\n")
	return b.String()
}

// execLine builds a systemd ExecStart line, quoting any token that contains a
// space so paths with spaces survive systemd's word splitting.
func execLine(program string, args []string) string {
	toks := append([]string{program}, args...)
	for i, t := range toks {
		if strings.ContainsAny(t, " \t") {
			toks[i] = `"` + t + `"`
		}
	}
	return strings.Join(toks, " ")
}

// systemdUnitDir is the per-user systemd unit directory, honoring
// XDG_CONFIG_HOME.
func systemdUnitDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(base, "systemd", "user")
}
