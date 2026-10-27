package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// a mistake in the arguments is a usage error, exit 2 with the usage line, and
// changes nothing. a cap is a number of dollars: zero clears it, a negative one
// or something that is not a number is a mistake, and so is a flag that does not
// exist, a second name, or a cap with no account to set it on.
func TestBudgetMistakesAreUsageErrorsAndChangeNothing(t *testing.T) {
	cases := [][]string{
		{"budget", "work", "--daily", "-5"},
		{"budget", "work", "--weekly", "-0.01"},
		{"budget", "work", "--daily", "NaN"},
		{"budget", "work", "--daily", "nan"},
		{"budget", "work", "--daily", "Inf"},
		{"budget", "work", "--weekly", "-Inf"},
		{"budget", "work", "--weekly", "+Inf"},
		{"budget", "work", "--daily", "abc"},
		{"budget", "work", "--daily", ""},
		{"budget", "work", "--daily"},
		{"budget", "work", "--weekly"},
		{"budget", "--daily", "5"},
		{"budget", "--clear"},
		{"budget", "work", "--dayly", "5"},
		{"budget", "--verbose"},
		{"budget", "work", "-d", "5"},
		{"budget", "work", "home", "--daily", "5"},
		{"budget", "work", "home"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			e.signIn("home", "h@example.com")
			e.run("budget", "work", "--daily", "20", "--weekly", "90")
			before := e.account("work").Meta
			e.reset()

			code := e.run(args...)

			if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt budget [name] [--daily N] [--weekly N] [--clear]") {
				t.Errorf("exit %d, stderr %q, want 2 and the usage line", code, e.errOut.String())
			}
			if strings.Contains(e.out.String(), "updated") {
				t.Errorf("it claimed an update: %q", e.out.String())
			}
			if after := e.account("work").Meta; after != before || e.account("home").Meta.DailyBudget != 0 {
				t.Errorf("a caps changed: %+v then %+v", before, after)
			}
		})
	}
}

func TestBudgetAcceptsZeroAndDecimals(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("budget", "work", "--daily", "12.5", "--weekly", "0"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if m := e.account("work").Meta; m.DailyBudget != 12.5 || m.WeeklyBudget != 0 {
		t.Errorf("meta = %+v", m)
	}
}

// a name with no cap is a look at the caps, and nothing was updated
func TestBudgetWithAnAccountAndNoCapJustShowsTheTable(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.run("budget", "work", "--daily", "20")
	e.reset()

	code := e.run("budget", "work")

	if code != 0 || strings.Contains(e.out.String(), "updated") || !strings.Contains(e.out.String(), "$20") {
		t.Errorf("exit %d, stdout %q", code, e.out.String())
	}
	if e.account("work").Meta.DailyBudget != 20 {
		t.Error("the cap changed")
	}
}

func TestBudgetOfSomeoneUnknownIsStillNotUsageButAnUnknownAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	code := e.run("budget", "nobody")

	if code != 1 || !strings.Contains(e.errOut.String(), `no account called "nobody"`) {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

func TestUsageMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"usage", "--verbose"},
		{"usage", "-x"},
		{"usage", "work", "--since", "1d"},
		{"usage", "work", "home"},
		{"usage", "--live"}, // retired, and answered before usage runs
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			e.signIn("home", "h@example.com")
			e.reset()

			code := e.run(args...)

			if code != 2 {
				t.Errorf("exit %d, stderr %q, stdout %q, want 2", code, e.errOut.String(), e.out.String())
			}
			if args[1] != "--live" && !strings.Contains(e.errOut.String(), "usage: clalt usage [name] [--json]") {
				t.Errorf("stderr %q has no usage line", e.errOut.String())
			}
			if e.out.Len() != 0 {
				t.Errorf("a report was printed anyway:\n%s", e.out.String())
			}
		})
	}
}

func TestUsageStillTakesOneNameAndTheJsonFlag(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")

	rows := usageRows(t, e, "work")

	if len(rows) != 1 || rows[0]["name"] != "work" {
		t.Errorf("rows = %v", rows)
	}
}

// json mode prints json and nothing else, also after a cap was set
func TestBudgetJsonAfterSettingACapIsOnlyJson(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("budget", "work", "--daily", "5", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil {
		t.Fatalf("the output is not json by itself: %v\n%s", err, e.out.String())
	}
	if len(rows) != 1 || rows[0]["dailyBudget"] != float64(5) {
		t.Errorf("rows = %v", rows)
	}
}
