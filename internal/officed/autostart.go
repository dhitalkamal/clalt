package officed

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Label is the launchd label of the start at login job.
const Label = "com.dhitalkamal.clalt.office"

// Autostart is the launchd job that starts the office at login (macOS only).
type Autostart struct {
	Home    string
	UID     int
	Exe     string // absolute path of the clalt binary
	LogPath string
	GOOS    string
	// PATH is the search path the job runs with, launchd gives jobs a bare one
	PATH string
	// Run is launchctl, args without the program name
	Run func(args []string) ([]byte, error)
}

// Supported says whether start at login exists on this system.
func (a Autostart) Supported() bool { return a.GOOS == "darwin" }

// PlistPath is the job file in the user's LaunchAgents.
func (a Autostart) PlistPath() string {
	return filepath.Join(a.Home, "Library", "LaunchAgents", Label+".plist")
}

// Enabled says whether the job file exists.
func (a Autostart) Enabled() bool {
	_, err := os.Stat(a.PlistPath())
	return err == nil
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// cleanPath splits a path list on ':', drops empty entries and repeats, and
// keeps the first-seen order.
func cleanPath(p string) string {
	seen := map[string]bool{}
	var out []string
	for _, e := range strings.Split(p, ":") {
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return strings.Join(out, ":")
}

// envBlock is the EnvironmentVariables entry, empty when there is no PATH.
func (a Autostart) envBlock() string {
	p := cleanPath(a.PATH)
	if p == "" {
		return ""
	}
	return fmt.Sprintf("\t<key>EnvironmentVariables</key>\n\t<dict>\n\t\t<key>PATH</key>\n\t\t<string>%s</string>\n\t</dict>\n", esc(p))
}

// Plist is the job file. it never holds the token: the server makes its own.
func (a Autostart) Plist() []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>office</string>
		<string>serve</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
%s</dict>
</plist>
`, Label, esc(a.Exe), esc(a.LogPath), esc(a.LogPath), a.envBlock()))
}

var errMacOnly = errors.New("start at login is macOS only for now")

func (a Autostart) launchctl(args ...string) error {
	out, err := a.Run(args)
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Enable writes the job and loads it. an already loaded job is replaced.
func (a Autostart) Enable() error {
	if !a.Supported() {
		return errMacOnly
	}
	path := a.PlistPath()
	if a.Enabled() {
		// bootstrap fails on a loaded job, a failed bootout only means it was not loaded
		_ = a.launchctl("bootout", fmt.Sprintf("gui/%d", a.UID), path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.LogPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, a.Plist(), 0o644); err != nil {
		return err
	}
	if err := a.launchctl("bootstrap", fmt.Sprintf("gui/%d", a.UID), path); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// Disable unloads the job (when unload is set) and removes the file. unload is
// false when the server asking is itself the launchd job, which would end it.
func (a Autostart) Disable(unload bool) error {
	if !a.Supported() {
		return errMacOnly
	}
	if unload && a.Enabled() {
		// not loaded is fine, the file goes either way
		_ = a.launchctl("bootout", fmt.Sprintf("gui/%d", a.UID), a.PlistPath())
	}
	if err := os.Remove(a.PlistPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
