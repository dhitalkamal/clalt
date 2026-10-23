package notify

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNoSettingsFileMeansOn(t *testing.T) {
	on, err := Enabled(t.TempDir())
	if !on || err != nil {
		t.Errorf("on %v, err %v", on, err)
	}
}

func TestOffRoundTrips(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notify")
	if err := SetEnabled(dir, false); err != nil {
		t.Fatal(err)
	}
	if on, err := Enabled(dir); on || err != nil {
		t.Errorf("after off: on %v, err %v", on, err)
	}
	if err := SetEnabled(dir, true); err != nil {
		t.Fatal(err)
	}
	if on, err := Enabled(dir); !on || err != nil {
		t.Errorf("after on: on %v, err %v", on, err)
	}
}

func TestSettingsFileAndDirModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no unix permission bits to check")
	}
	dir := filepath.Join(t.TempDir(), "notify")
	SetEnabled(dir, false)
	di, err := os.Stat(dir)
	if err != nil || di.Mode().Perm() != 0o700 {
		t.Fatalf("dir: %v %v", di, err)
	}
	fi, err := os.Stat(filepath.Join(dir, "settings.json"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("file: %v %v", fi, err)
	}
}

func TestBadSettingsMeanOnWithAnError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{nope"), 0o600)
	on, err := Enabled(dir)
	if !on || err == nil {
		t.Errorf("invalid json: on %v, err %v", on, err)
	}
	os.Chmod(filepath.Join(dir, "settings.json"), 0o000)
	if os.Getuid() != 0 {
		on, err = Enabled(dir)
		if !on || err == nil {
			t.Errorf("unreadable: on %v, err %v", on, err)
		}
	}
}
