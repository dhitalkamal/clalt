package cli

import (
	"encoding/json"
	"testing"
)

// with --json the table is the whole of stdout and stderr stays empty, in every
// form of the command. the line that says a cap was set is a line of text, so
// json mode leaves it out: in front of the table it would make the output
// something no json reader takes, and a script that reads stdout has no use for
// it
func TestBudgetJsonIsAllThatStdoutHoldsAndStderrStaysEmpty(t *testing.T) {
	forms := []struct {
		name   string
		args   []string
		daily  float64
		weekly float64
	}{
		{"every account", []string{"budget", "--json"}, 0, 0},
		{"one account", []string{"budget", "work", "--json"}, 0, 0},
		{"a daily cap", []string{"budget", "work", "--daily", "5", "--json"}, 5, 0},
		{"a weekly cap, with the flag first", []string{"--json", "budget", "work", "--weekly", "9"}, 0, 9},
		{"both caps, with the flag in the middle", []string{"budget", "work", "--daily", "5", "--json", "--weekly", "9"}, 5, 9},
		{"clearing the caps", []string{"budget", "work", "--clear", "--json"}, 0, 0},
	}
	for _, f := range forms {
		t.Run(f.name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			e.signIn("home", "h@example.com")
			// spend to report, so that the transcripts are read too
			e.writeSwitchLog(switchToWork)
			e.writeTranscript(replyLine)

			code := e.run(f.args...)

			if code != 0 {
				t.Fatalf("exit %d: %s", code, e.errOut.String())
			}
			if e.errOut.Len() != 0 {
				t.Errorf("stderr is not empty: %q", e.errOut.String())
			}
			var rows []map[string]any
			if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil {
				t.Fatalf("stdout is not json by itself: %v\n%s", err, e.out.String())
			}
			work := rowNamed(rows, "work")
			if len(rows) != 2 || work == nil || work["dailyBudget"] != f.daily || work["weeklyBudget"] != f.weekly {
				t.Errorf("rows = %v, want two accounts and the caps %v and %v on work", rows, f.daily, f.weekly)
			}
		})
	}
}
