package officed

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleRun() Run {
	return Run{PID: 4242, Port: 47321, Token: "tok123", StartedAt: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), Version: "0.2.0"}
}

func TestRunFileRoundTripAndModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "office")
	if err := WriteRun(dir, sampleRun()); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "run.json"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v %v", fi, err)
	}
	di, _ := os.Stat(dir)
	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %v", di.Mode().Perm())
	}
	got, err := ReadRun(dir)
	if err != nil || got != sampleRun() {
		t.Errorf("got %+v %v", got, err)
	}
}

func TestReadRunMissingAndBroken(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadRun(dir); !errors.Is(err, ErrNoRun) {
		t.Errorf("missing: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "run.json"), []byte("{nope"), 0o600)
	if _, err := ReadRun(dir); !errors.Is(err, ErrNoRun) {
		t.Errorf("broken file counts as no run: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "run.json"), []byte(`{"pid":0,"port":0}`), 0o600)
	if _, err := ReadRun(dir); !errors.Is(err, ErrNoRun) {
		t.Errorf("empty fields count as no run: %v", err)
	}
}

func TestRemoveRunOnlyIfPidMatches(t *testing.T) {
	dir := t.TempDir()
	WriteRun(dir, sampleRun())
	RemoveRun(dir, 1)
	if _, err := ReadRun(dir); err != nil {
		t.Fatalf("another pid removed the file: %v", err)
	}
	RemoveRun(dir, 4242)
	if _, err := ReadRun(dir); !errors.Is(err, ErrNoRun) {
		t.Errorf("own pid did not remove: %v", err)
	}
	WriteRun(dir, sampleRun())
	RemoveRun(dir, 0)
	if _, err := ReadRun(dir); !errors.Is(err, ErrNoRun) {
		t.Errorf("pid 0 means any: %v", err)
	}
}

func TestLink(t *testing.T) {
	if got := sampleRun().Link(); got != "http://127.0.0.1:47321/?t=tok123" {
		t.Errorf("link %q", got)
	}
}

func TestPickPort(t *testing.T) {
	busy := map[int]bool{100: true, 101: true}
	free := func(p int) bool { return !busy[p] }
	if p, err := PickPort(100, 5, free); err != nil || p != 102 {
		t.Errorf("got %d %v", p, err)
	}
	if p, _ := PickPort(103, 5, free); p != 103 {
		t.Errorf("free start: %d", p)
	}
	if _, err := PickPort(100, 2, free); err == nil {
		t.Error("all busy must fail")
	}
}

func TestRotateLog(t *testing.T) {
	p := filepath.Join(t.TempDir(), "office.log")
	os.WriteFile(p, make([]byte, 100), 0o600)
	RotateLog(p, 1000)
	if _, err := os.Stat(p + ".old"); err == nil {
		t.Fatal("small log rotated")
	}
	os.WriteFile(p, make([]byte, 2000), 0o600)
	os.WriteFile(p+".old", []byte("older"), 0o600)
	if err := RotateLog(p, 1000); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Stat(p + ".old")
	if _, err := os.Stat(p); err == nil || old == nil || old.Size() != 2000 {
		t.Errorf("rotation did not keep exactly one old file")
	}
}

func TestLastErrorLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "office.log")
	if LastErrorLine(p) != "" {
		t.Error("no file")
	}
	os.WriteFile(p, []byte("a ok\nb snapshot failed: x\nc fine\n"), 0o600)
	if got := LastErrorLine(p); got != "b snapshot failed: x" {
		t.Errorf("got %q", got)
	}
}
