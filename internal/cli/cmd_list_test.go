package cli

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func listJSON(t *testing.T, e *testEnv) []map[string]any {
	t.Helper()
	e.reset()
	if code := e.run("list", "--json"); code != 0 {
		t.Fatalf("list --json exited %d: %s", code, e.errOut.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	return rows
}

func TestListWithNoAccountsSaysHowToStart(t *testing.T) {
	e := newEnv(t)

	if code := e.run("list"); code != 0 || !strings.Contains(e.out.String(), "clalt login <name>") {
		t.Errorf("exit %d:\n%s", code, e.out.String())
	}
	if rows := listJSON(t, e); rows == nil || len(rows) != 0 {
		t.Errorf("json = %v, want an empty list and not null", rows)
	}
}

func TestListShowsEachVisibleAccountWithItsLiveState(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	e.run("logout", "home")
	e.signIn("gone", "g@example.com")
	e.run("remove", "gone", "--force")
	e.reset()

	code := e.run("list")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	out := e.out.String()
	for _, want := range []string{"LABEL", "w@example.com", "max", "yes", dateOf(loginDue(theClock)), "h@example.com", "no: not signed in", "never"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "gone") {
		t.Errorf("a removed account is listed:\n%s", out)
	}
	if strings.Index(out, "home") > strings.Index(out, "work") {
		t.Errorf("accounts should be sorted by label:\n%s", out)
	}
}

func TestListJSONHasTheAgreedFields(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	e.run("logout", "home")

	rows := listJSON(t, e)

	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	home, work := rows[0], rows[1]
	for _, key := range []string{"id", "label", "email", "org", "plan", "signedIn", "reason", "signedInAt", "loginDueAt", "lastUsedAt", "folder"} {
		if _, ok := work[key]; !ok {
			t.Errorf("key %q is missing from %v", key, work)
		}
	}
	if work["id"] != "work" || work["label"] != "work" || work["email"] != "w@example.com" || work["org"] != "Example Org" ||
		work["plan"] != "max" || work["signedIn"] != true || work["reason"] != "" || work["folder"] != e.folder("work") {
		t.Errorf("work = %v", work)
	}
	if work["signedInAt"] != theClock.Format(time.RFC3339) || work["loginDueAt"] != loginDue(theClock).Format(time.RFC3339) || work["lastUsedAt"] != nil {
		t.Errorf("dates = %v / %v / %v", work["signedInAt"], work["loginDueAt"], work["lastUsedAt"])
	}
	if home["signedIn"] != false || home["reason"] != "not signed in" || home["signedInAt"] != nil || home["loginDueAt"] != nil {
		t.Errorf("home = %v: signed out, no dates", home)
	}
}

func TestListRecordsLastUse(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.run("run", "work")

	rows := listJSON(t, e)

	if rows[0]["lastUsedAt"] == nil {
		t.Errorf("lastUsedAt is null after a run: %v", rows[0])
	}
}

func TestListSaysUnknownWhenTheCheckFails(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("STATUS_BROKEN", "1")

	row := listJSON(t, e)[0]
	if row["signedIn"] != nil || !strings.Contains(row["reason"].(string), "keychain is locked") {
		t.Errorf("row = %v, want signedIn null and the reason", row)
	}
	e.reset()
	e.run("list")
	if out := e.out.String(); !strings.Contains(out, "unknown: ") || !strings.Contains(out, "keychain is locked") {
		t.Errorf("output:\n%s", out)
	}
}

func TestListDoesNotWaitForeverForClaude(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("STATUS_HANGS", "1")
	old := authTimeout
	authTimeout = 300 * time.Millisecond
	t.Cleanup(func() { authTimeout = old })

	started := time.Now()
	row := listJSON(t, e)[0]

	if time.Since(started) > 5*time.Second {
		t.Error("list waited for a claude that never answered")
	}
	if row["signedIn"] != nil || row["reason"] == "" {
		t.Errorf("row = %v, want unknown with a reason", row)
	}
}

func TestListSaysSoWhenTheFolderIsSignedInAsSomeoneElse(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.plantLogin(e.folder("work"), "other@example.com")

	row := listJSON(t, e)[0]

	if row["signedIn"] != false || !strings.Contains(row["reason"].(string), "different account") {
		t.Errorf("row = %v", row)
	}
}

func TestListShowsAccountsFromV010UnderTheirId(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"work","accounts":{"work":{"email":"w@example.com","plan":"max","addedAt":"2026-08-01T10:00:00Z"}}}`)

	row := listJSON(t, e)[0]

	if row["label"] != "work" || row["email"] != "w@example.com" || row["signedIn"] != false {
		t.Errorf("row = %v: a v0.1.0 account has no folder yet, so it is signed out until it logs in", row)
	}
}

func TestListTakesNoArguments(t *testing.T) {
	e := newEnv(t)
	if code := e.run("list", "work"); code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt list") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestParallelDoRunsEverythingOnceWithinTheLimit(t *testing.T) {
	var inFlight, peak, runs atomic.Int32
	seen := make([]atomic.Int32, 20)

	parallelDo(20, 4, func(i int) {
		now := inFlight.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		seen[i].Add(1)
		runs.Add(1)
		inFlight.Add(-1)
	})

	if runs.Load() != 20 {
		t.Errorf("ran %d times, want 20", runs.Load())
	}
	for i := range seen {
		if seen[i].Load() != 1 {
			t.Errorf("index %d ran %d times", i, seen[i].Load())
		}
	}
	if peak.Load() > 4 || peak.Load() < 2 {
		t.Errorf("peak concurrency = %d, want between 2 and 4", peak.Load())
	}
}

func TestAgo(t *testing.T) {
	now := theClock
	cases := map[time.Duration]string{
		0:                      "just now",
		30 * time.Second:       "just now",
		5 * time.Minute:        "5m ago",
		3 * time.Hour:          "3h ago",
		49 * time.Hour:         "2d ago",
		100 * 24 * time.Hour:   dateOf(now.Add(-100 * 24 * time.Hour)),
		-2 * time.Minute:       "just now",
		90*time.Minute + 1:     "1h ago",
		23*time.Hour + 59*60e9: "23h ago",
		24 * time.Hour:         "1d ago",
		59 * 24 * time.Hour:    "59d ago",
		60 * 24 * time.Hour:    dateOf(now.Add(-60 * 24 * time.Hour)),
	}
	for d, want := range cases {
		if got := ago(now.Add(-d), now); got != want {
			t.Errorf("ago(%v before now) = %q, want %q", d, got, want)
		}
	}
	if got := ago(time.Time{}, now); got != "never" {
		t.Errorf("ago(zero) = %q, want never", got)
	}
}
