package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type countingWriter struct {
	writes int
	buf    bytes.Buffer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	return w.buf.Write(p)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("the pipe is closed") }

// a report written line by line can stop half way and still exit 0, so the text
// is built first and written once.
func TestMigrateWritesTheTextReportOnce(t *testing.T) {
	profiles, _ := migrateHome(t)
	twoTranscripts(t, profiles)
	app, _, _ := newTestApp(t)
	w := &countingWriter{}
	app.Out, app.Err = w, &bytes.Buffer{}

	if code := run(app, []string{"migrate"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if w.writes != 1 || !strings.Contains(w.buf.String(), "total:") {
		t.Errorf("%d writes of %q, want one write of the whole report", w.writes, w.buf.String())
	}
}

func TestMigrateReturnsTheWriteError(t *testing.T) {
	for _, args := range [][]string{{"migrate"}, {"migrate", "--dry-run"}, {"migrate", "--json"}} {
		profiles, _ := migrateHome(t)
		twoTranscripts(t, profiles)
		app, _, _ := newTestApp(t)
		errOut := &bytes.Buffer{}
		app.Out, app.Err = failingWriter{}, errOut

		code := run(app, args)
		if code != 1 || !strings.Contains(errOut.String(), "the pipe is closed") {
			t.Errorf("%v: exit %d, stderr %q, want exit 1 and the write error", args, code, errOut.String())
		}
	}
}
