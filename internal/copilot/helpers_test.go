package copilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 7, 18, 0, 0, 0, time.UTC)

const marker = "SECRET-MARKER-TEXT"

// ev builds one events.jsonl line. free text fields carry the marker so the
// tests can prove none of it comes back out.
func ev(typ string, data map[string]any) string {
	if data == nil {
		data = map[string]any{}
	}
	b, _ := json.Marshal(map[string]any{"type": typ, "data": data, "id": "x", "timestamp": "2026-11-07T17:00:00.000Z"})
	return string(b)
}

func start(id, cwd string) string {
	return ev("session.start", map[string]any{"sessionId": id, "startTime": "2026-11-07T17:00:00.000Z", "context": map[string]any{"cwd": cwd}, "copilotVersion": "1.0.107"})
}

func prompt() string {
	return ev("user.message", map[string]any{"content": marker, "transformedContent": marker})
}

func model(name string) string {
	return ev("session.model_change", map[string]any{"newModel": name})
}

// fixture makes a copilot home with a clean slate.
type fixture struct {
	t    *testing.T
	home string
	r    *Reader
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return &fixture{t: t, home: t.TempDir()}
}

func (f *fixture) reader() *Reader {
	if f.r == nil {
		f.r = &Reader{Home: f.home, Now: func() time.Time { return now }}
	}
	return f.r
}

// session writes a session dir with the lines and an events mtime of age ago.
func (f *fixture) session(id string, age time.Duration, lines ...string) string {
	f.t.Helper()
	dir := filepath.Join(f.home, "session-state", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.touch(id, age)
	return dir
}

func (f *fixture) touch(id string, age time.Duration) {
	f.t.Helper()
	mt := now.Add(-age)
	if err := os.Chtimes(filepath.Join(f.home, "session-state", id, "events.jsonl"), mt, mt); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) list() []Session {
	f.t.Helper()
	got, err := f.reader().Sessions()
	if err != nil {
		f.t.Fatal(err)
	}
	return got
}
