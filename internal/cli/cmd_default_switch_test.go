package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// transcriptOf is a reply of session id at ts, 3000 tokens
func transcriptOf(id string, ts time.Time) string {
	return strings.Replace(replyAfter(ts.Format(time.RFC3339), id, 3000), `"sessionId":"s1"`, `"sessionId":"`+id+`"`, 1)
}

func ledgerAccounts(t *testing.T) map[string]string {
	t.Helper()
	lines, _ := usage.ReadSessionAccounts(config.SessionAccountsPath())
	out := map[string]string{}
	for _, l := range lines {
		out[l.SessionID] = l.Account
	}
	return out
}

func TestSwitchingTheDefaultLoginOnlyChargesTheNewAccountForWhatCameAfter(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "a@example.com")
	e.signIn("side", "b@example.com")
	work, side := e.account("work").Name, e.account("side").Name
	e.plainClaudeAs("a@example.com")
	e.app.refreshDefaultHome()
	e.writeSwitchLog(switchToWork)
	t1 := loginTime.Add(2 * time.Hour)
	e.writeTranscript(transcriptOf("sA", loginTime.Add(time.Hour)), transcriptOf("sB", t1.Add(time.Hour)))

	// clalt confirms the match again after sA, then the user logs in as the other
	e.at(loginTime.Add(90 * time.Minute))
	e.app.refreshDefaultHome()
	e.plainClaudeAs("b@example.com")
	e.at(t1)
	c, _ := e.app.refreshDefaultHome()
	if c.Account != side || !c.MatchedSince.Equal(t1) {
		t.Fatalf("%+v", c)
	}
	e.at(t1.Add(2 * time.Hour))
	usageByName(t, e)

	got := ledgerAccounts(t)
	if got["sA"] != work || got["sB"] != side {
		t.Errorf("ledger: %v", got)
	}
}

func TestMatchingAfterBeingOutsideDoesNotBackfill(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "a@example.com")
	e.plainClaudeAs("other@example.com")
	e.app.refreshDefaultHome()
	e.writeSwitchLog(switchToWork)
	t1 := loginTime.Add(2 * time.Hour)
	e.writeTranscript(transcriptOf("old", loginTime.Add(time.Hour)), transcriptOf("new", t1.Add(time.Hour)))
	e.plainClaudeAs("a@example.com")
	e.at(t1)
	e.app.refreshDefaultHome()
	e.at(t1.Add(2 * time.Hour))

	usageByName(t, e)

	got := ledgerAccounts(t)
	if _, ok := got["old"]; ok || got["new"] == "" {
		t.Errorf("ledger: %v", got)
	}
}

func TestALoginSwitchBetweenTwoChecksIsNotChargedToTheOldAccount(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "a@example.com")
	e.signIn("side", "b@example.com")
	e.plainClaudeAs("a@example.com")
	e.app.refreshDefaultHome() // the last time A was confirmed
	e.writeSwitchLog(switchToWork)
	// started after that check: A or B, nobody knows
	e.writeTranscript(transcriptOf("mid", loginTime.Add(time.Hour)))
	e.plainClaudeAs("b@example.com")
	e.at(loginTime.Add(2 * time.Hour))
	e.app.refreshDefaultHome()
	e.at(loginTime.Add(3 * time.Hour))

	usageByName(t, e)

	if got := ledgerAccounts(t); got["mid"] != "" {
		t.Errorf("charged: %v", got)
	}
}
