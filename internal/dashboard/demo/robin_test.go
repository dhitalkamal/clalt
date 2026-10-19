package demo

import (
	"fmt"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// the numbers below are the approved frames. where the ledger could not keep
// one of them the test says what changed.

// ago is how long before the demo's clock a time is
func ago(t time.Time) time.Duration { return Epoch.Sub(t) }

func TestRobinsDaysMatchTheApprovedTable(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	days := a.History.Days
	labels := []string{"Wed 15", "Tue 14", "Mon 13", "Sun 12", "Sat 11", "Fri 10", "Thu 9", "Wed 8", "Tue 7", "Mon 6"}
	costs := []float64{2.14, 3.62, 2.87, 0.95, 1.40, 4.05, 3.37, 2.70, 3.10, 2.55}
	sessions := []int{3, 4, 3, 1, 2, 5, 4, 3, 4, 3}
	input := []string{"0.4", "0.7", "0.5", "0.2", "0.3", "0.8", "0.6", "0.5", "0.6", "0.5"}
	output := []string{"0.1", "0.2", "0.2", "0.1", "0.1", "0.2", "0.2", "0.2", "0.2", "0.2"}
	cacheRd := []string{"6.2", "10.5", "8.3", "2.8", "4.1", "11.7", "9.8", "7.8", "9.0", "7.4"}
	cacheWr := []string{"0.3", "0.5", "0.4", "0.1", "0.2", "0.6", "0.5", "0.4", "0.4", "0.4"}
	millions := func(n int64) string { return fmt.Sprintf("%.1f", float64(n)/1e6) }
	for i := range labels {
		d := days[i]
		if d.Label != labels[i] || toCents(d.Cost) != toCents(costs[i]) || d.Sessions != sessions[i] {
			t.Errorf("row %d = %s %.2f %d sessions, want %s %.2f %d", i, d.Label, d.Cost, d.Sessions, labels[i], costs[i], sessions[i])
		}
		got := []string{millions(d.Input), millions(d.Output), millions(d.CacheRead), millions(d.CacheWrite)}
		want := []string{input[i], output[i], cacheRd[i], cacheWr[i]}
		for k := range got {
			if got[k] != want[k] {
				t.Errorf("%s: token column %d reads %sM, approved %sM", labels[i], k, got[k], want[k])
			}
		}
	}
}

func TestRobinsTotalsMatchTheApprovedFrames(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	days := a.History.Days
	if got := toCents(days[0].Cost); got != 214 {
		t.Errorf("today = %d cents, want 214", got)
	}
	var week, month int
	for i, d := range days {
		month += toCents(d.Cost)
		if i < 7 {
			week += toCents(d.Cost)
		}
	}
	if week != 1840 || month != 7120 {
		t.Errorf("week %d and 30 day %d cents, want 1840 and 7120", week, month)
	}
	// the approved overview said 1.2M and 9.8M tokens, which no cut of the days
	// can produce. the headline is input plus output, as the usage package
	// counts it, so today is 529k and the week 4.5M.
	var weekTokens int64
	for _, d := range days[:7] {
		weekTokens += d.Input + d.Output
	}
	if today := days[0].Input + days[0].Output; today != 528_580 || weekTokens < 4_500_000 || weekTokens >= 4_600_000 {
		t.Errorf("headline tokens today %d and week %d", today, weekTokens)
	}
}

func TestRobinsProjectsAndModelsKeepTheApprovedCosts(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	wantProjects := []struct {
		name  string
		cents int
		pct   int
	}{{"clalt", 4710, 66}, {"web", 926, 13}, {"portal", 641, 9}, {"site", 427, 6}, {"infra", 214, 3}, {"other", 202, 3}}
	if len(a.History.Projects) != len(wantProjects) {
		t.Fatalf("projects = %+v", a.History.Projects)
	}
	for i, w := range wantProjects {
		p := a.History.Projects[i]
		if p.Name != w.name || toCents(p.Cost) != w.cents {
			t.Errorf("project %d = %s %.2f, want %s %.2f", i, p.Name, p.Cost, w.name, float64(w.cents)/100)
		}
		if got := int(float64(p.Cost)/71.20*100 + 0.5); got != w.pct {
			t.Errorf("%s share = %d%%, approved %d%%", w.name, got, w.pct)
		}
	}
	wantModels := []struct {
		name  string
		cents int
	}{{"Opus 5.5", 4910}, {"Sonnet 5.5", 2210}}
	for i, w := range wantModels {
		m := a.History.Models[i]
		if m.Name != w.name || toCents(m.Cost) != w.cents {
			t.Errorf("model %d = %s %.2f, want %s %.2f", i, m.Name, m.Cost, w.name, float64(w.cents)/100)
		}
	}
	// the cheaper model carries more tokens per dollar, which is why the models
	// table calls its share column cost share
	opus, sonnet := a.History.Models[0], a.History.Models[1]
	if float64(sonnet.Tokens)/sonnet.Cost <= float64(opus.Tokens)/opus.Cost {
		t.Errorf("sonnet should carry more tokens per dollar: %d for %.2f against %d for %.2f", sonnet.Tokens, sonnet.Cost, opus.Tokens, opus.Cost)
	}
}

func TestRobinsRunningSessionsMatchTheApprovedFrame(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	if len(a.Sessions) != 2 {
		t.Fatalf("sessions = %+v", a.Sessions)
	}
	w, q := a.Sessions[0], a.Sessions[1]
	if w.State != dashboard.StateWorking || w.Folder != "~/Projects/personal/clalt" || w.Branch != "feat/dashboard-v2" ||
		w.Model != "Opus 5.5" || w.Effort != "high" || w.ContextUsed != 312_000 || w.ContextSize != 1_000_000 ||
		ago(w.StartedAt).Truncate(time.Minute) != 2*time.Hour+4*time.Minute || w.Added != 156 || w.Removed != 23 {
		t.Errorf("working session = %+v", w)
	}
	if q.State != dashboard.StateWaiting || q.Folder != "~/Projects/work/web" || q.Branch != "develop" ||
		q.Model != "Sonnet 5.5" || q.Effort != "medium" || q.ContextUsed != 48_000 || q.ContextSize != 1_000_000 ||
		ago(q.StartedAt).Truncate(time.Minute) != 40*time.Minute || q.Added != 12 || q.Removed != 3 {
		t.Errorf("waiting session = %+v", q)
	}
	// the approved frame said ~$4.10, more than all of today's $2.14, so the
	// working session now costs $1.10. the waiting session keeps $0.62.
	if toCents(w.Cost) != 110 || toCents(q.Cost) != 62 {
		t.Errorf("session costs %.2f and %.2f, want 1.10 and 0.62", w.Cost, q.Cost)
	}
}

func TestRobinsLimitsAndIdentityMatchTheApprovedFrames(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	if a.Label != "robin" || a.Email != "robin@example.com" || a.Org != "Example" || a.Plan != "team plan" || !a.SignedIn {
		t.Errorf("identity = %+v", a)
	}
	signedDays := int(ago(a.Login.SignedInAt) / day)
	dueDays := int((a.Login.DueAt.Sub(Epoch) + day - 1) / day)
	if signedDays != 7 || dueDays != 23 || ago(a.LastUsedAt).Truncate(time.Minute) != 4*time.Minute || a.LastIn != "~/Projects/personal/clalt" {
		t.Errorf("login and last used = %+v %v %q", a.Login, a.LastUsedAt, a.LastIn)
	}
	f, s := a.Limits.Five, a.Limits.Seven
	if f.Percent != 26 || f.ResetsAt.Format("15:04") != "01:45" || f.ResetsAt.Sub(Epoch)/time.Minute != 112 ||
		f.RunsOutAt.Format("15:04") != "01:10" || f.ResetsAt.Sub(f.RunsOutAt) != 35*time.Minute || f.Projected < 100 {
		t.Errorf("5h = %+v", f)
	}
	if s.Percent != 42 || s.ResetsAt.Format("Mon 15:04") != "Fri 15:45" || s.ResetsAt.Sub(Epoch)/time.Hour != 39 ||
		!s.RunsOutAt.IsZero() || s.Projected != 71 {
		t.Errorf("7d = %+v", s)
	}
	// the approved frame was taken four seconds after the load, so the reading
	// is stamped at the load itself
	if !a.Limits.ReadAt.Equal(Epoch) || a.Budget.Daily != 10 || a.Budget.Weekly != 50 {
		t.Errorf("limits read at %v budget %+v", a.Limits.ReadAt, a.Budget)
	}
	p := a.Profile
	if p.Folder != "~/.config/clalt/profiles/robin" || p.ClaudeVersion != "2.1.296" || !p.SharedSettings || !p.FeedInstalled {
		t.Errorf("profile = %+v", p)
	}
}

func TestTheListMatchesTheApprovedAccounts(t *testing.T) {
	snap := loadSnapshot(t)
	want := []struct {
		id     string
		signed bool
		five   int
	}{{"robin", true, 26}, {"morgan", true, 22}, {"casey", true, 95}, {"jordan", true, 5}, {"harriet", false, 0}}
	if len(snap.Accounts) != len(want) {
		t.Fatalf("%d accounts", len(snap.Accounts))
	}
	for i, w := range want {
		a := snap.Accounts[i]
		if a.ID != w.id || a.Label != w.id || a.SignedIn != w.signed || a.Limits.Five.Percent != w.five {
			t.Errorf("account %d = %s %v %d%%, want %s %v %d%%", i, a.ID, a.SignedIn, a.Limits.Five.Percent, w.id, w.signed, w.five)
		}
	}
	casey := snap.Accounts[2]
	if len(casey.Sessions) != 1 || casey.Sessions[0].State != dashboard.StateWaiting {
		t.Errorf("casey has one waiting session, got %+v", casey.Sessions)
	}
	if casey.Budget.Daily != 8 || casey.Budget.Weekly != 40 {
		t.Errorf("casey's budget = %+v, approved $8 a day and $40 a week", casey.Budget)
	}
	// the 5h window runs out in minutes and the week ends just under the cap
	if five := casey.Limits.Five; five.RunsOutAt.Sub(Epoch) != 5*time.Minute+15*time.Second || five.ResetsAt.Sub(five.RunsOutAt) != 43*time.Minute+15*time.Second {
		t.Errorf("casey's 5h window = %+v", five)
	}
	if seven := casey.Limits.Seven; seven.Projected != 97 || !seven.RunsOutAt.IsZero() {
		t.Errorf("casey's 7d window = %+v", seven)
	}
}

func TestOtherOverviewTokenTotalsKeepTheirPreviewText(t *testing.T) {
	snap := loadSnapshot(t)
	headline := func(days []dashboard.Day) int64 {
		var n int64
		for _, d := range days {
			n += d.Input + d.Output
		}
		return n
	}
	cents := func(days []dashboard.Day) int {
		var n int
		for _, d := range days {
			n += toCents(d.Cost)
		}
		return n
	}
	cases := []struct {
		id                    string
		today, week           int64
		todayCents, weekCents int
	}{
		// casey's overview is in the approved frames, the others were in the preview
		{"casey", 3_400_000, 16_200_000, 680, 3320},
		{"morgan", 700_000, 5_100_000, 135, 960},
		{"jordan", 0, 1_100_000, 0, 205},
	}
	for _, c := range cases {
		days := accountByID(t, snap, c.id).History.Days
		if got := headline(days[:1]); got != c.today {
			t.Errorf("%s today = %d tokens, want %d", c.id, got, c.today)
		}
		if got := headline(days[:7]); got != c.week {
			t.Errorf("%s week = %d tokens, want %d", c.id, got, c.week)
		}
		if got := cents(days[:1]); got != c.todayCents {
			t.Errorf("%s today = %d cents, want %d", c.id, got, c.todayCents)
		}
		if got := cents(days[:7]); got != c.weekCents {
			t.Errorf("%s week = %d cents, want %d", c.id, got, c.weekCents)
		}
	}
}
