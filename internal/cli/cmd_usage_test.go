package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// a reply of 300 tokens, written the way claude writes it: the same line once
// per content block.
const replyLine = `{"type":"assistant","timestamp":"2026-08-24T10:00:00Z","sessionId":"s1","requestId":"req_1","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":100,"output_tokens":200}}}` + "\n"

// writeTranscript puts transcript lines where claude keeps them in the shared
// home.
func (e *testEnv) writeTranscript(lines ...string) {
	e.t.Helper()
	dir := filepath.Join(config.ClaudeProjectsDir(), "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(strings.Join(lines, "")), 0o600); err != nil {
		e.t.Fatal(err)
	}
}

// writeSwitchLog writes the log an old clalt kept: from the time of an event on,
// usage belongs to the account the event names.
func (e *testEnv) writeSwitchLog(events ...string) {
	e.t.Helper()
	if err := os.MkdirAll(config.ClaltDir(), 0o700); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(config.SwitchLogPath(), []byte(strings.Join(events, "\n")+"\n"), 0o600); err != nil {
		e.t.Fatal(err)
	}
}

const switchToWork = `{"ts":"2026-08-01T00:00:00Z","account":"work"}`

func usageRows(t *testing.T, e *testEnv, args ...string) []map[string]any {
	t.Helper()
	e.reset()
	if code := e.run(append([]string{"usage", "--json"}, args...)...); code != 0 {
		t.Fatalf("usage exited %d: %s", code, e.errOut.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	return rows
}

func rowNamed(rows []map[string]any, name string) map[string]any {
	for _, r := range rows {
		if r["name"] == name {
			return r
		}
	}
	return nil
}

func TestUsageReportsEachVisibleAccountByLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := e.app.Store.Relabel("work", "office"); err != nil {
		t.Fatal(err)
	}
	e.signIn("gone", "g@example.com")
	e.run("remove", "gone", "--force")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(replyLine)

	rows := usageRows(t, e)

	if len(rows) != 1 || rows[0]["name"] != "office" || rows[0]["id"] != "work" || rows[0]["tokens"].(float64) != 300 {
		t.Errorf("rows = %v, want the one visible account, by its label, with 300 tokens", rows)
	}
}

func TestUsageNobodyCanBeBlamedForShowsAsUnattributed(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.writeTranscript(replyLine) // no switch log at all

	rows := usageRows(t, e)

	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	if work := rowNamed(rows, "work"); work == nil || work["tokens"].(float64) != 0 {
		t.Errorf("work = %v, want no usage", work)
	}
	if un := rowNamed(rows, "unattributed"); un == nil || un["tokens"].(float64) != 300 {
		t.Errorf("unattributed = %v, want the 300 tokens", un)
	}
	e.reset()
	e.run("usage")
	if out := e.out.String(); !strings.Contains(out, "== unattributed ==") || !strings.Contains(out, "work") {
		t.Errorf("output:\n%s", out)
	}
}

// a script that keys the rows by id finds what nobody can be blamed for there
// too, instead of one row that has none
func TestTheUnattributedRowHasAnIDLikeTheOthers(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.writeTranscript(replyLine)

	rows := usageRows(t, e)

	un := rowNamed(rows, "unattributed")
	if un == nil || un["id"] != "unattributed" {
		t.Errorf("unattributed = %v, want id unattributed", un)
	}
	if work := rowNamed(rows, "work"); work == nil || work["id"] != "work" {
		t.Errorf("work = %v, want id work", work)
	}
}

func TestUsageCountsAReplyWrittenAsSeveralLinesOnce(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(replyLine, replyLine)

	rows := usageRows(t, e)

	if got := rowNamed(rows, "work")["tokens"].(float64); got != 300 {
		t.Errorf("tokens = %v, want 300 (one reply, not two)", got)
	}
}

func TestUsageOfOneAccountTakesALabelWhateverTheCase(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	e.writeTranscript(replyLine)

	rows := usageRows(t, e, "WORK")

	if len(rows) != 1 || rows[0]["name"] != "work" {
		t.Errorf("rows = %v, want only work and no unattributed row", rows)
	}
}

func TestUsageOfSomeoneUnknownOrRemovedIsAnError(t *testing.T) {
	e := newEnv(t)
	e.signIn("gone", "g@example.com")
	e.run("remove", "gone", "--force")
	for _, label := range []string{"nobody", "gone"} {
		e.reset()
		if code := e.run("usage", label); code != 1 || e.errOut.Len() == 0 {
			t.Errorf("%s: exit %d, stderr %q", label, code, e.errOut.String())
		}
	}
}

func TestUsageHasNoTokenOrLiveFields(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	row := usageRows(t, e)[0]

	for _, gone := range []string{"active", "expiresAt", "live", "liveNote"} {
		if _, ok := row[gone]; ok {
			t.Errorf("field %q should be gone: %v", gone, row)
		}
	}
	e.reset()
	e.run("usage")
	out := e.out.String()
	for _, gone := range []string{"token:", "live", "(active)", "--live"} {
		if strings.Contains(out, gone) {
			t.Errorf("the text still mentions %q:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "== work ==") {
		t.Errorf("output:\n%s", out)
	}
}

func TestUsageWithNoAccountsSaysSo(t *testing.T) {
	e := newEnv(t)

	if code := e.run("usage"); code != 0 || !strings.Contains(e.out.String(), "no accounts") {
		t.Errorf("exit %d:\n%s", code, e.out.String())
	}
}
