package feedinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefusals(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		r := newRig(t, `{"model": `)
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
			t.Errorf("%v", err)
		}
		if r.read() != `{"model": ` {
			t.Error("changed")
		}
	})
	t.Run("not an object", func(t *testing.T) {
		r := newRig(t, `[1]`)
		if _, err := Prepare(r.opts); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("hooks of the wrong shape", func(t *testing.T) {
		r := newRig(t, `{"hooks":{"SessionStart":"x"}}`)
		if _, err := Prepare(r.opts); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("a directory", func(t *testing.T) {
		r := newRig(t, "")
		os.MkdirAll(r.settings, 0o755)
		if _, err := Prepare(r.opts); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Errorf("%v", err)
		}
	})
}

func TestApplyAbortsWhenTheFileChangedAfterPrepare(t *testing.T) {
	r := newRig(t, userFile)
	p, err := Prepare(r.opts)
	if err != nil {
		t.Fatal(err)
	}
	// claude rewrote the file while the question was on screen
	changed := strings.Replace(userFile, `"opus"`, `"sonnet"`, 1)
	if err := os.WriteFile(r.settings, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}

	err = p.Apply()

	if err == nil || !strings.Contains(err.Error(), "settings.json changed while waiting, run clalt feed install again") {
		t.Fatalf("%v", err)
	}
	if r.read() != changed {
		t.Error("the change was overwritten")
	}
	if _, err := os.Stat(r.opts.StatePath); err == nil {
		t.Error("state written for an install that did not happen")
	}
}

func TestApplyAbortsWhenTheFileAppearedAfterPrepare(t *testing.T) {
	r := newRig(t, "")
	p, _ := Prepare(r.opts)
	os.WriteFile(r.settings, []byte(`{"a":1}`), 0o600)

	if err := p.Apply(); err == nil {
		t.Error("overwrote a file that was not there at Prepare")
	}
}

func TestAFailedWriteKeepsThePreviousInstallsState(t *testing.T) {
	if isWindows() || os.Getuid() == 0 {
		t.Skip("needs a folder that refuses writes")
	}
	r := newRig(t, userFile)
	r.install()
	before, _ := os.ReadFile(r.opts.StatePath)
	// a second pass with something to add: drop one of our events from the file
	// by hand, then make the folder read only
	stripped := strings.Replace(r.read(), "PostToolUse", "PostToolUseX", 1)
	os.WriteFile(r.settings, []byte(stripped), 0o600)
	p, err := Prepare(r.opts)
	if err != nil || !p.Changed {
		t.Fatalf("%v %v", p.Changed, err)
	}
	dir := filepath.Dir(r.settings)
	os.Chmod(dir, 0o500)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := p.Apply(); err == nil {
		t.Fatal("the write worked in a read only folder")
	}

	after, err := os.ReadFile(r.opts.StatePath)
	if err != nil || string(after) != string(before) {
		t.Errorf("state lost or changed: %v", err)
	}
}
