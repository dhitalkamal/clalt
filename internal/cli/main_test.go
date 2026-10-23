package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helperFakeOK says whether a helper process was handed a fake it may run as
// claude: a program named claude, claude-guard or their .exe form, inside the
// temp folder. an exported variable of the developer's cannot pass for it.
func helperFakeOK(fake, tmp string) error {
	if fake == "" {
		return fmt.Errorf("a helper process needs CLALT_TEST_FAKE")
	}
	base := strings.TrimSuffix(filepath.Base(fake), ".exe")
	if base != "claude" && base != "claude-guard" {
		return fmt.Errorf("CLALT_TEST_FAKE %q is not a fake claude", fake)
	}
	if !strings.HasPrefix(fake, tmp) {
		return fmt.Errorf("CLALT_TEST_FAKE %q is outside %s", fake, tmp)
	}
	return nil
}

// TestMain builds the fakes and points CLALT_CLAUDE_BIN at the guard, which
// keeps the real claude out of reach of every test, including one that forgot
// its fake.
func TestMain(m *testing.M) {
	// a helper process of a test is handed its fake and needs no build
	if os.Getenv("CLALT_TEST_RUN_HELPER") != "" {
		if err := helperFakeOK(os.Getenv("CLALT_TEST_FAKE"), os.TempDir()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Setenv("CLALT_CLAUDE_BIN", os.Getenv("CLALT_TEST_FAKE"))
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "clalt-cli-guard-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	if err := buildFakes(dir, "fakeclaude", "fakekirocrew"); err != nil {
		fail(err)
	}
	guard, err := installFake("fakeclaude", dir, "claude-guard")
	if err != nil {
		fail(err)
	}
	os.Setenv("CLALT_CLAUDE_BIN", guard)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
