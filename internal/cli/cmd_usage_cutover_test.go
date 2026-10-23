package cli

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// the switch log an old clalt kept stopped growing when accounts became folders.
// usage from the first sign-in of the new design on is nobody's yet, and is
// shown as unattributed until something records whose it is, instead of being
// charged to the account of the last old event. that goes for the budget caps
// too, which are the numbers people act on.
var (
	loginTime = theClock                                       // 2026-10-22 12:00 UTC
	beforeIt  = replyLine                                      // 2026-08-24, 300 tokens
	afterIt   = replyAfter("2026-10-23T10:00:00Z", "r2", 3000) // the day after the sign-in
)

// replyAfter is a reply like replyLine at another time, under its own ids, with
// total input and output tokens, so that two replies can be told apart.
func replyAfter(timestamp, id string, total int) string {
	return strings.NewReplacer(
		"2026-08-24T10:00:00Z", timestamp,
		"req_1", "req_"+id,
		"msg_1", "msg_"+id,
		`"input_tokens":100,"output_tokens":200`, `"input_tokens":`+strconv.Itoa(total/3)+`,"output_tokens":`+strconv.Itoa(total-total/3),
	).Replace(replyLine)
}

func usageByName(t *testing.T, e *testEnv) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, row := range usageRows(t, e) {
		out[row["name"].(string)] = row
	}
	return out
}

func TestUsageFromTheFirstSignInOnIsUnattributed(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "w@example.com")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(beforeIt, afterIt)

	rows := usageByName(t, e)

	if got := rows["work"]["tokens"]; got != float64(300) {
		t.Errorf("work tokens = %v, want 300: only what came before the sign-in", got)
	}
	if got := rows["unattributed"]["tokens"]; got != float64(3000) {
		t.Errorf("unattributed tokens = %v, want 3000: what came after", got)
	}
}

func TestUsageBeforeAnySignInIsStillAttributedByTheLog(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"work","accounts":{"work":{"email":"w@example.com","plan":"max","addedAt":"2026-08-01T10:00:00Z"}}}`)
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(beforeIt, afterIt)

	rows := usageByName(t, e)

	if got := rows["work"]["tokens"]; got != float64(3300) {
		t.Errorf("work tokens = %v, want 3300: with no sign-in of the new design the log is all there is", got)
	}
	if _, ok := rows["unattributed"]; ok {
		t.Errorf("something was left unattributed: %v", rows["unattributed"])
	}
}

func TestABudgetCapDoesNotSeeUsageThatIsNobodysYet(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "w@example.com")
	e.run("budget", "work", "--daily", "0.0001")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(afterIt)
	e.at(time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))

	rows := budgetRows(t, e)

	if rows[0]["todayCost"] != float64(0) || rows[0]["overDaily"] != false {
		t.Errorf("row = %v: usage after the first sign-in is not work's", rows[0])
	}
}

func TestTheDashboardAttributesTheSameWay(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "w@example.com")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(beforeIt, afterIt)

	byAccount, err := (&dashboardHost{app: e.app}).Usage()

	if err != nil {
		t.Fatal(err)
	}
	if recs := byAccount["work"]; len(recs) != 1 || recs[0].Billable() != 300 {
		t.Errorf("the dashboard reads %+v for work, want the one reply of 300 tokens", recs)
	}
}
