package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

type settings struct {
	Enabled *bool `json:"enabled"`
	// Idle turns on the notification for a session that is idle and waits for
	// the next prompt. off when absent.
	Idle *bool `json:"idle,omitempty"`
}

func settingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

// load reads the settings file in dir. no file is the zero settings.
func load(dir string) (settings, error) {
	var s settings
	data, err := fsutil.ReadFile(settingsPath(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read notify settings: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return settings{}, fmt.Errorf("parse %s: %w", settingsPath(dir), err)
	}
	return s, nil
}

// Enabled reads the on/off setting in dir. no file means on. a file that
// cannot be read or parsed also means on, with an error that says why.
func Enabled(dir string) (bool, error) {
	s, err := load(dir)
	return s.Enabled == nil || *s.Enabled, err
}

// IdleEnabled reads the setting for idle prompt notifications in dir. it is off
// unless set, also when the file cannot be read.
func IdleEnabled(dir string) (bool, error) {
	s, err := load(dir)
	return s.Idle != nil && *s.Idle, err
}

// SetEnabled writes the on/off setting in dir.
func SetEnabled(dir string, on bool) error {
	return update(dir, func(s *settings) { s.Enabled = &on })
}

// SetIdleEnabled writes the idle prompt setting in dir.
func SetIdleEnabled(dir string, on bool) error {
	return update(dir, func(s *settings) { s.Idle = &on })
}

// update changes one setting and keeps the others. a file that cannot be parsed
// is replaced.
func update(dir string, fn func(*settings)) error {
	s, _ := load(dir)
	fn(&s)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(settingsPath(dir), append(data, '\n'), 0o600)
}
