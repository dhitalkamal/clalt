package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultLinesClaimOnlyUnattributedSessionsAfterTheCutover(t *testing.T) {
	ledger := []SessionAccount{{Ts: cut.Add(time.Hour), SessionID: "run", Account: "other"}}
	records := []Record{
		sessRec("plain", cut.Add(3*time.Hour)),
		sessRec("plain", cut.Add(2*time.Hour)), // the earliest sets Ts
		sessRec("run", cut.Add(2*time.Hour)),   // a clalt run session has its line
		sessRec("old", cut.Add(-time.Hour)),    // before the cutover: the switch log's
		sessRec("", cut.Add(2*time.Hour)),      // no id, nothing to key a line on
	}

	got := DefaultLines(records, ledger, cut, "work")

	if len(got) != 1 {
		t.Fatalf("lines: %+v", got)
	}
	l := got[0]
	if l.SessionID != "plain" || l.Account != "work" || l.Source != SourceDefault || !l.Ts.Equal(cut.Add(2*time.Hour)) {
		t.Errorf("line: %+v", l)
	}
}

func TestDefaultLinesSkipASessionThatStartedBeforeTheCutover(t *testing.T) {
	records := []Record{
		sessRec("straddle", cut.Add(-time.Hour)), // started before the login of the account
		sessRec("straddle", cut.Add(time.Hour)),
		sessRec("after", cut.Add(time.Hour)),
	}

	got := DefaultLines(records, nil, cut, "work")

	if len(got) != 1 || got[0].SessionID != "after" {
		t.Errorf("lines: %+v", got)
	}
}

func TestDefaultLinesDoNothingWithoutACutoverOrAnAccount(t *testing.T) {
	recs := []Record{sessRec("plain", cut.Add(time.Hour))}
	if got := DefaultLines(recs, nil, time.Time{}, "work"); len(got) != 0 {
		t.Errorf("no cutover: %+v", got)
	}
	if got := DefaultLines(recs, nil, cut, ""); len(got) != 0 {
		t.Errorf("no account: %+v", got)
	}
}

func TestDefaultLinesMakeAttributeSessionsChargeTheAccountAndLeaveOthersAlone(t *testing.T) {
	ledger := []SessionAccount{{Ts: cut.Add(time.Hour), SessionID: "run", Account: "other"}}
	records := []Record{sessRec("plain", cut.Add(2*time.Hour)), sessRec("run", cut.Add(2*time.Hour))}

	ledger = append(ledger, DefaultLines(records, ledger, cut, "work")...)
	got := AttributeSessions(records, nil, ledger, cut)

	if len(got["work"]) != 1 || got["work"][0].SessionID != "plain" || len(got["other"]) != 1 || len(got[Unattributed]) != 0 {
		t.Errorf("work %d other %d unattributed %d", len(got["work"]), len(got["other"]), len(got[Unattributed]))
	}
}

func TestSourceSurvivesTheLedgerAndForgetRemovesOnlyDefaultLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session-accounts.jsonl")
	lines := []SessionAccount{
		{Ts: cut, SessionID: "a", Account: "w"},
		{Ts: cut, SessionID: "b", Account: "w", Source: SourceDefault},
		{Ts: cut, SessionID: "c", Account: "x", Source: SourceDefault},
	}
	for _, l := range lines {
		if err := AppendSessionAccount(p, l); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := os.ReadFile(p)
	if strings.Count(string(raw), `"source":"default"`) != 2 || strings.Contains(strings.SplitN(string(raw), "\n", 2)[0], "source") {
		t.Errorf("ledger: %s", raw)
	}

	n, err := RemoveSource(p, SourceDefault)

	if err != nil || n != 2 {
		t.Fatalf("removed %d, %v", n, err)
	}
	left, _ := ReadSessionAccounts(p)
	if len(left) != 1 || left[0].SessionID != "a" {
		t.Errorf("left: %+v", left)
	}
	if n, err := RemoveSource(filepath.Join(t.TempDir(), "none"), SourceDefault); n != 0 || err != nil {
		t.Errorf("missing ledger: %d %v", n, err)
	}
}
