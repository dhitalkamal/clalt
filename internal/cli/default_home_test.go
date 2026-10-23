package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/defaulthome"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// plainClaudeAs signs the default home (~/.claude, what plain claude uses) in
// as email, with the fake.
func (e *testEnv) plainClaudeAs(email string) {
	e.t.Helper()
	e.claude.plantLogin(filepath.Join(e.home, ".claude"), email)
}

func cachedDefaultHome(t *testing.T) defaulthome.Cache {
	t.Helper()
	c, _ := defaulthome.Read(config.DefaultHomePath())
	return c
}

func TestRefreshMatchesTheDefaultHomeByEmailIgnoringCase(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "Me@Example.com")
	e.plainClaudeAs("me@example.COM")

	c, err := e.app.refreshDefaultHome()

	if err != nil || c.State != defaulthome.Matched || c.Account != e.account("work").Name {
		t.Fatalf("%+v %v", c, err)
	}
	if got := cachedDefaultHome(t); got.Account != c.Account {
		t.Errorf("not cached: %+v", got)
	}
}

func TestRefreshLeavesPlainClaudeOutsideWhenTheEmailIsNobodys(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	e.plainClaudeAs("someone@else.com")

	c, _ := e.app.refreshDefaultHome()

	if c.State != defaulthome.Outside || c.Account != "" || c.Reason == "" {
		t.Errorf("%+v", c)
	}
}

func TestRefreshWithAnUnusableClaudeKeepsTheEarlierMatch(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	e.plainClaudeAs("me@example.com")
	e.app.refreshDefaultHome()
	e.claude.set("STATUS_BROKEN", "1")

	c, _ := e.app.refreshDefaultHome()

	if c.State != defaulthome.Matched || c.Reason == "" {
		t.Errorf("%+v", c)
	}
}

func TestRefreshSignedOutDefaultHomeIsOutside(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")

	c, _ := e.app.refreshDefaultHome()

	if c.State != defaulthome.Outside || c.Account != "" {
		t.Errorf("%+v", c)
	}
}

func TestUsageOfSessionsClaltNeverSawGoesToTheMatchedAccountAndSaysSo(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "me@example.com")
	e.signIn("side", "side@example.com")
	e.plainClaudeAs("me@example.com")
	e.app.refreshDefaultHome()
	e.writeSwitchLog(switchToWork)
	// the old reply goes by the switch log, the new session by the default home
	e.writeTranscript(beforeIt, transcriptOf("s2", loginTime.Add(time.Hour)))

	rows := usageByName(t, e)

	if got := rows["work"]["tokens"]; got != float64(3300) {
		t.Errorf("work tokens = %v", got)
	}
	if _, ok := rows["unattributed"]; ok {
		t.Errorf("unattributed left: %v", rows["unattributed"])
	}
	lines, _ := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if len(lines) != 1 || lines[0].SessionID != "s2" || lines[0].Source != usage.SourceDefault || lines[0].Account != e.account("work").Name {
		t.Errorf("ledger: %+v", lines)
	}
	// a second report adds nothing
	usageByName(t, e)
	if again, _ := usage.ReadSessionAccounts(config.SessionAccountsPath()); len(again) != 1 {
		t.Errorf("ledger grew: %+v", again)
	}
}

func TestUsageAlreadyAttributedByAClaltRunSessionIsNotTouched(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "me@example.com")
	e.signIn("side", "side@example.com")
	e.plainClaudeAs("me@example.com")
	e.app.refreshDefaultHome()
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(afterIt)
	err := usage.AppendSessionAccount(config.SessionAccountsPath(), usage.SessionAccount{
		Ts: loginTime.Add(time.Hour), SessionID: "s1", Account: e.account("side").Name,
	})
	if err != nil {
		t.Fatal(err)
	}

	rows := usageByName(t, e)

	if rows["side"]["tokens"] != float64(3000) || rows["work"]["tokens"] != float64(0) {
		t.Errorf("side %v work %v", rows["side"]["tokens"], rows["work"]["tokens"])
	}
}

func TestUsageStaysUnattributedWithoutAMatchOrWhenAttributionIsOff(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "me@example.com")
	e.plainClaudeAs("someone@else.com")
	e.app.refreshDefaultHome()
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(afterIt)

	if got := usageByName(t, e)["unattributed"]["tokens"]; got != float64(3000) {
		t.Errorf("no match: %v", got)
	}

	e.plainClaudeAs("me@example.com")
	e.app.refreshDefaultHome()
	c := cachedDefaultHome(t)
	c.Off = true
	if err := defaulthome.Write(config.DefaultHomePath(), c); err != nil {
		t.Fatal(err)
	}
	if got := usageByName(t, e)["unattributed"]["tokens"]; got != float64(3000) {
		t.Errorf("attribution off: %v", got)
	}
}
