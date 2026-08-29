// Package launchd builds and manages a macOS LaunchAgent so clalt can keep
// account tokens refreshed in the background, whether or not clalt is otherwise
// in use. It uses StartInterval (periodic one-shot) rather than a long-lived
// process so a crash never leaves refresh silently dead.
package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Agent describes a LaunchAgent to install.
type Agent struct {
	Label      string   // reverse-DNS label, e.g. com.clalt.keepalive
	Program    string   // absolute path to the clalt binary
	Args       []string // args passed after Program (e.g. keepalive --once)
	Interval   int      // seconds between runs (StartInterval)
	StdoutPath string   // where launchd writes stdout
	StderrPath string   // where launchd writes stderr
	RunAtLoad  bool     // also run once immediately on load
}

// Plist renders the LaunchAgent as an XML property list. Program is emitted as
// the first ProgramArguments entry, followed by each arg in order.
func (a Agent) Plist() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")

	b.WriteString("  <key>Label</key>\n")
	fmt.Fprintf(&b, "  <string>%s</string>\n", xmlEscape(a.Label))

	b.WriteString("  <key>ProgramArguments</key>\n")
	b.WriteString("  <array>\n")
	fmt.Fprintf(&b, "    <string>%s</string>\n", xmlEscape(a.Program))
	for _, arg := range a.Args {
		fmt.Fprintf(&b, "    <string>%s</string>\n", xmlEscape(arg))
	}
	b.WriteString("  </array>\n")

	b.WriteString("  <key>StartInterval</key>\n")
	fmt.Fprintf(&b, "  <integer>%d</integer>\n", a.Interval)

	if a.RunAtLoad {
		b.WriteString("  <key>RunAtLoad</key>\n")
		b.WriteString("  <true/>\n")
	}
	if a.StdoutPath != "" {
		b.WriteString("  <key>StandardOutPath</key>\n")
		fmt.Fprintf(&b, "  <string>%s</string>\n", xmlEscape(a.StdoutPath))
	}
	if a.StderrPath != "" {
		b.WriteString("  <key>StandardErrorPath</key>\n")
		fmt.Fprintf(&b, "  <string>%s</string>\n", xmlEscape(a.StderrPath))
	}

	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

// PlistPath is the per-user LaunchAgents location for a given label.
func PlistPath(label string) string {
	return filepath.Join(homeDir(), "Library", "LaunchAgents", label+".plist")
}

// Install writes the plist and (re)loads it via launchctl. It is idempotent:
// an already-loaded agent is unloaded first so the new definition takes effect.
func Install(a Agent) error {
	path := PlistPath(a.Label)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(a.Plist()), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	// unload first (ignore error if it was not loaded), then load fresh
	_ = run("launchctl", "unload", path)
	if err := run("launchctl", "load", path); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

// Uninstall unloads the agent and removes its plist. A not-loaded agent or a
// missing file is not treated as an error.
func Uninstall(label string) error {
	path := PlistPath(label)
	_ = run("launchctl", "unload", path)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

// Status reports whether the agent is currently loaded in launchd.
func Status(label string) (loaded bool, err error) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return false, fmt.Errorf("launchctl list: %w", err)
	}
	return strings.Contains(string(out), label), nil
}

func run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return os.Getenv("HOME")
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
