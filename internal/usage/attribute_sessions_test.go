package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	cut = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	day = 24 * time.Hour
)

func sessRec(session string, at time.Time) Record { return Record{SessionID: session, Timestamp: at} }

func TestAttributeSessionsUsesTheSwitchLogBeforeTheCutoverAndSessionsAfter(t *testing.T) {
	switches := []SwitchEvent{{Ts: cut.Add(-10 * day), Account: "old"}}
	sessions := []SessionAccount{{Ts: cut.Add(time.Hour), SessionID: "s1", Account: "work"}}
	records := []Record{
		sessRec("s0", cut.Add(-day)),        // before: the log
		sessRec("s1", cut.Add(2*time.Hour)), // after, mapped
		sessRec("s2", cut.Add(2*time.Hour)), // after, nobody knows
		sessRec("", cut.Add(2*time.Hour)),   // after, no session id
	}

	got := AttributeSessions(records, switches, sessions, cut)

	if len(got["old"]) != 1 || len(got["work"]) != 1 || len(got[Unattributed]) != 2 {
		t.Errorf("old %d work %d unattributed %d", len(got["old"]), len(got["work"]), len(got[Unattributed]))
	}
}

func TestAttributeSessionsFollowsAConversationThatMovedAccounts(t *testing.T) {
	sessions := []SessionAccount{
		{Ts: cut.Add(1 * time.Hour), SessionID: "s", Account: "a"},
		{Ts: cut.Add(5 * time.Hour), SessionID: "s", Account: "b"},
	}
	records := []Record{
		sessRec("s", cut.Add(30*time.Minute)), // before the first mapping: the account it started under
		sessRec("s", cut.Add(2*time.Hour)),
		sessRec("s", cut.Add(6*time.Hour)),
	}

	got := AttributeSessions(records, nil, sessions, cut)

	if len(got["a"]) != 2 || len(got["b"]) != 1 {
		t.Errorf("a %d b %d", len(got["a"]), len(got["b"]))
	}
}

func TestAttributeSessionsWithNoCutoverIsTheOldLogAlone(t *testing.T) {
	switches := []SwitchEvent{{Ts: cut.Add(-day), Account: "old"}}
	sessions := []SessionAccount{{Ts: cut, SessionID: "s", Account: "work"}}

	got := AttributeSessions([]Record{sessRec("s", cut.Add(day))}, switches, sessions, time.Time{})

	if len(got["old"]) != 1 {
		t.Errorf("%v", got)
	}
}

func TestReadSessionAccountsSkipsBadLinesAndAMissingFile(t *testing.T) {
	dir := t.TempDir()
	if got, err := ReadSessionAccounts(filepath.Join(dir, "none.jsonl")); err != nil || len(got) != 0 {
		t.Errorf("%v %v", got, err)
	}
	path := filepath.Join(dir, "m.jsonl")
	body := `{"ts":"2026-10-22T13:00:00Z","sessionId":"s1","account":"work"}` + "\nnot json\n" +
		`{"ts":"2026-10-22T14:00:00Z","sessionId":"","account":"x"}` + "\n"
	_ = os.WriteFile(path, []byte(body), 0o600)

	got, err := ReadSessionAccounts(path)

	if err != nil || len(got) != 1 || got[0].SessionID != "s1" || got[0].Account != "work" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestAppendSessionAccountWritesOneLineEach(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "m.jsonl")
	for _, s := range []string{"a", "b"} {
		if err := AppendSessionAccount(path, SessionAccount{Ts: cut, SessionID: s, Account: "work"}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := ReadSessionAccounts(path)
	if len(got) != 2 || got[1].SessionID != "b" {
		t.Errorf("%+v", got)
	}
}

// an over-long line is skipped, and the lines after it still count.
func TestReadSessionAccountsSkipsAnOversizedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	body := `{"ts":"2026-10-01T00:00:00Z","sessionId":"s1","account":"a"}` + "\n" +
		strings.Repeat("x", 200*1024) + "\n" +
		"not json\n" +
		`{"ts":"2026-10-02T00:00:00Z","sessionId":"s2","account":"b"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadSessionAccounts(path)

	if err != nil || len(got) != 2 || got[0].SessionID != "s1" || got[1].SessionID != "s2" {
		t.Errorf("got %v, err %v", got, err)
	}
}
