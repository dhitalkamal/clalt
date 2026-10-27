package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// v0.1.0 kept two small files in clalt's folder for the keepalive job: one when
// it set the job up by itself, one when the user turned it off. nothing reads
// them now, on any platform.
func plantMarkers(t *testing.T, names ...string) []string {
	t.Helper()
	if err := os.MkdirAll(config.ClaltDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, name := range names {
		path := filepath.Join(config.ClaltDir(), name)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestDoctorListsTheOldKeepaliveMarkersAndLeavesThem(t *testing.T) {
	e := newEnv(t)
	_, rep := doctor(t, e)
	rep.want(t, "leftover", "oldKeepaliveMarkers", "ok", "")

	paths := plantMarkers(t, "keepalive.auto", "keepalive.disabled")
	code, rep := doctor(t, e)

	for _, path := range paths {
		rep.want(t, "leftover", "oldKeepaliveMarkers", "warn", path)
		if !exists(path) {
			t.Errorf("doctor deleted %s", path)
		}
	}
	if code != 0 {
		t.Errorf("a leftover is a warning, exit %d", code)
	}
}

func TestTheMarkerCheckPointsAtTheWindowsTaskOnlyWhereItCouldBeThere(t *testing.T) {
	newEnv(t)
	const hint = "schtasks /Delete /TN com.clalt.keepalive /F"

	plantMarkers(t, "keepalive.auto")
	if got := markersCheck("windows"); got.Status != statusWarn || !strings.Contains(got.Detail, hint) {
		t.Errorf("windows with the auto marker: %+v", got)
	}
	if got := markersCheck("darwin"); strings.Contains(got.Detail, "schtasks") {
		t.Errorf("a task is a windows thing: %+v", got)
	}

	if err := os.Remove(filepath.Join(config.ClaltDir(), "keepalive.auto")); err != nil {
		t.Fatal(err)
	}
	plantMarkers(t, "keepalive.disabled")
	if got := markersCheck("windows"); got.Status != statusWarn || strings.Contains(got.Detail, "schtasks") {
		t.Errorf("the user turned the job off, so there is no task to remove: %+v", got)
	}
}
