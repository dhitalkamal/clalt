package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

// newTestApp builds an App on state files in a temp dir, with one buffer for
// both output streams. tests that go through the dispatcher and a fake claude
// use newEnv instead.
func newTestApp(t *testing.T) (*App, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	out := &bytes.Buffer{}
	app := &App{
		Store: account.New(filepath.Join(dir, "state.json"), filepath.Join(dir, "switches.jsonl")),
		Out:   out, Err: out,
	}
	return app, out, dir
}
