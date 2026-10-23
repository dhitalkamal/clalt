package claudecli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// guardScript is what CLALT_CLAUDE_BIN points at unless a test installs a fake
// of its own. it keeps the real claude out of reach of every test in this
// package, including one that forgot to set up a fake.
const guardScript = "#!/bin/sh\necho 'a test ran claude without installing a fake' >&2\nexit 99\n"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "clalt-claudecli-guard-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	guard := filepath.Join(dir, "claude")
	if err := os.WriteFile(guard, []byte(guardScript), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("CLALT_CLAUDE_BIN", guard)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
