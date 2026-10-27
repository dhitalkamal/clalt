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
</dict>
</plist>
`, Label, esc(a.Exe), esc(a.LogPath), esc(a.LogPath)))
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
