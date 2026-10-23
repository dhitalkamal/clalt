package claudecli

import (
	"context"
	"strings"
	"testing"
)

func TestAuthCallsRefuseAClaudeThatIsTooOld(t *testing.T) {
	f := newFake(t)
	f.say("version", 0, "2.1.300 (Claude Code)\n")
	f.say("status", 1, signedOutJSON)
	dir := t.TempDir()

	for call, err := range authCalls(dir) {
		if err == nil || !strings.Contains(err.Error(), "2.1.300") || !strings.Contains(err.Error(), MinVersion) {
			t.Errorf("%s: err = %v, want one that names 2.1.300 and %s", call, err, MinVersion)
		}
	}
	if f.ran("auth") != 0 {
		t.Error("an auth command ran against a claude that is too old")
	}
}

func TestAuthCallsRefuseAVersionTheyCannotRead(t *testing.T) {
	f := newFake(t)
	f.say("version", 0, "nightly build\n")
	f.say("status", 1, signedOutJSON)

	for call, err := range authCalls(t.TempDir()) {
		if err == nil || !strings.Contains(err.Error(), "nightly build") || !strings.Contains(err.Error(), MinVersion) {
			t.Errorf("%s: err = %v, want one that quotes the output and names %s", call, err, MinVersion)
		}
	}
	if f.ran("auth") != 0 {
		t.Error("an auth command ran without a readable version")
	}
}

func TestTheVersionIsCheckedOncePerBinary(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	dir := t.TempDir()

	for i := 0; i < 3; i++ {
		if _, err := Status(context.Background(), dir); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Login(context.Background(), dir, ""); err != nil {
		t.Fatal(err)
	}

	if n := f.ran("--version"); n != 1 {
		t.Errorf("claude --version ran %d times, want once", n)
	}
	if n := f.ran("auth status"); n != 3 {
		t.Errorf("auth status ran %d times, want 3", n)
	}
}

func TestAFailedVersionCheckIsNotRemembered(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	f.say("version", 0, "garbage")
	dir := t.TempDir()
	if _, err := Status(context.Background(), dir); err == nil {
		t.Fatal("expected the first call to fail")
	}

	// claude was upgraded in the meantime
	f.say("version", 0, "2.1.303 (Claude Code)\n")

	if _, err := Status(context.Background(), dir); err != nil {
		t.Errorf("the second call still failed: %v", err)
	}
}

func TestEachBinaryIsCheckedOnItsOwn(t *testing.T) {
	old := newFake(t)
	old.say("version", 0, "2.0.0 (Claude Code)\n")
	old.say("status", 1, signedOutJSON)
	dir := t.TempDir()
	if _, err := Status(context.Background(), dir); err == nil {
		t.Fatal("the old binary was accepted")
	}

	current := newFake(t)
	current.say("status", 1, signedOutJSON)
	if _, err := Status(context.Background(), dir); err != nil {
		t.Errorf("a different, current binary was refused: %v", err)
	}
}
