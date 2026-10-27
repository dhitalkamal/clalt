package cli

import (
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/usage"
)

func TestHookNotesWhichAccountEachClaudeSessionRunsUnder(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)

	e.hook("SessionStart", `{"session_id":"s1"}`)
	e.hook("Stop", `{"session_id":"s1"}`)
	e.hook("SessionStart", `{"session_id":"s2","source":"clear"}`)

	got, err := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if err != nil || len(got) != 2 || got[0].SessionID != "s1" || got[0].Account != "work" || got[1].SessionID != "s2" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestUsageAfterTheFirstSignInFollowsTheSessionsOfClaltRun(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "w@example.com")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(beforeIt, afterIt) // both are session s1
	err := usage.AppendSessionAccount(config.SessionAccountsPath(), usage.SessionAccount{
		Ts: loginTime.Add(time.Hour), SessionID: "s1", Account: e.account("work").Name,
	})
	if err != nil {
		t.Fatal(err)
	}

	rows := usageByName(t, e)

	if got := rows["work"]["tokens"]; got != float64(3300) {
		t.Errorf("work tokens = %v, want 3300: the old part by the log, the new part by the session", got)
	}
	if _, ok := rows["unattributed"]; ok {
		t.Errorf("something is left unattributed: %v", rows["unattributed"])
	}
}
