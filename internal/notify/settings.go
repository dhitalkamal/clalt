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
}

func settingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

// Enabled reads the on/off setting in dir. no file means on. a file that
// cannot be read or parsed also means on, with an error that says why.
func Enabled(dir string) (bool, error) {
	data, err := fsutil.ReadFile(settingsPath(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("read notify settings: %w", err)
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return true, fmt.Errorf("parse %s: %w", settingsPath(dir), err)
	}
	return s.Enabled == nil || *s.Enabled, nil
}

// SetEnabled writes the on/off setting in dir.
func SetEnabled(dir string, on bool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(settings{Enabled: &on})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(settingsPath(dir), append(data, '\n'), 0o600)
}
