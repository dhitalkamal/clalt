package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func budgetRows(t *testing.T, e *testEnv, args ...string) []map[string]any {
	t.Helper()
	e.reset()
	if code := e.run(append([]string{"budget", "--json"}, args...)...); code != 0 {
		t.Fatalf("budget exited %d: %s", code, e.errOut.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	return rows
}

func TestBudgetSetsCapsByLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := e.app.Store.Relabel("work", "office"); err != nil {
		t.Fatal(err)
	}

	code := e.run("budget", "OFFICE", "--daily", "20", "--weekly", "100")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	m := e.account("office").Meta
	if m.DailyBudget != 20 || m.WeeklyBudget != 100 {
		t.Errorf("meta = %+v", m)
	}
	out := e.out.String()
	for _, want := range []string{"budget updated for office", "$20", "$100"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestBudgetListsVisibleAccountsByLabelWithTheirSpend(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	e.signIn("gone", "g@example.com")
	e.run("remove", "gone", "--force")
	// a reply from earlier on the day of the sign-in, so that it counts as today's
	// spend and was written before the log stopped being the way to tell whose it is
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(`{"type":"assistant","timestamp":"` + theClock.Add(-time.Hour).Format(time.RFC3339) +
		`","sessionId":"s1","requestId":"r1","message":{"id":"m1","model":"claude-sonnet-5","usage":{"input_tokens":2000000,"output_tokens":1000000}}}` + "\n")
	e.run("budget", "work", "--daily", "0.01")

	rows := budgetRows(t, e)

	if len(rows) != 1 || rows[0]["name"] != "work" || rows[0]["id"] != "work" {
		t.Fatalf("rows = %v, want only the visible account", rows)
	}
	if rows[0]["dailyBudget"].(float64) != 0.01 || rows[0]["todayCost"].(float64) <= 0.01 || rows[0]["overDaily"] != true {
		t.Errorf("row = %v: spend above the cap should say so", rows[0])
	}
}

func TestBudgetClearRemovesBothCaps(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.run("budget", "work", "--daily", "5", "--weekly", "9")

	if code := e.run("budget", "work", "--clear"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if m := e.account("work").Meta; m.DailyBudget != 0 || m.WeeklyBudget != 0 {
		t.Errorf("meta = %+v", m)
	}
}

func TestBudgetRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"nobody", []string{"budget", "nobody", "--daily", "1"}, 1, "no account called"},
		{"a removed account", []string{"budget", "gone", "--daily", "1"}, 1, "was removed"},
		{"a flag without a number", []string{"budget", "work", "--daily"}, 2, "needs a number"},
		{"a number that is not one", []string{"budget", "work", "--weekly", "lots"}, 2, "bad number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			e.signIn("gone", "g@example.com")
			e.run("remove", "gone", "--force")
			e.reset()

			code := e.run(c.args...)

			if code != c.code || !strings.Contains(e.errOut.String(), c.want) {
				t.Errorf("exit %d: %s, want %d and %q", code, e.errOut.String(), c.code, c.want)
			}
		})
	}
}
