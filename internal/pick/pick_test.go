package pick

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func acct(id string) Candidate { return Candidate{ID: id, Label: id, SignedIn: true} }

// seen is limits observed a minute ago, with these used percentages.
func seen(five, seven float64) Limits {
	return Limits{
		FiveHour: win(five, 2*time.Hour), SevenDay: win(seven, 72*time.Hour),
		ObservedAt: now.Add(-time.Minute),
	}
}

func options(cwd string, byID map[string]Limits) Options {
	return Options{
		Cwd: cwd, Now: now,
		Limits: func(id string) (Limits, bool) { l, ok := byID[id]; return l, ok },
	}
}

func ids(r Result) []string {
	out := []string{}
	for _, c := range r.Ranked {
		out = append(out, c.ID)
	}
	return out
}

func TestTheAccountWithTheMostRoomComesFirst(t *testing.T) {
	r := Rank([]Candidate{acct("a"), acct("b"), acct("c")}, options("/x", map[string]Limits{
		"a": seen(50, 10), "b": seen(10, 10), "c": seen(30, 10),
	}))

	if got := ids(r); !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
		t.Errorf("order = %v, want b c a", got)
	}
	if r.Ranked[0].Room != 90 || !r.Ranked[0].HasData {
		t.Errorf("first = %+v", r.Ranked[0])
	}
}

func TestRoomIsTheHigherOfTheTwoWindows(t *testing.T) {
	// a has the emptier five-hour window and a nearly used week
	r := Rank([]Candidate{acct("a"), acct("b")}, options("/x", map[string]Limits{
		"a": seen(5, 90), "b": seen(40, 40),
	}))

	if got := ids(r); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Errorf("order = %v, want b a", got)
	}
}

func TestAccountsWithoutRecentDataRankAfterThoseWithAndGoByLabel(t *testing.T) {
	old := seen(0, 0)
	old.ObservedAt = now.Add(-DefaultMaxAge - time.Minute)
	cands := []Candidate{acct("zed"), acct("Beta"), acct("alpha"), acct("old"), acct("busy")}

	r := Rank(cands, options("/x", map[string]Limits{"busy": seen(95, 10), "old": old}))

	// busy has data, so it is first even though it has almost no room left
	if got := ids(r); !reflect.DeepEqual(got, []string{"busy", "alpha", "Beta", "old", "zed"}) {
		t.Errorf("order = %v", got)
	}
	if r.Ranked[1].HasData {
		t.Error("an account with no data is marked as having some")
	}
}

func TestWithNoDataAnywhereItIsLabelOrder(t *testing.T) {
	r := Rank([]Candidate{acct("b"), acct("A"), acct("c")}, options("/x", nil))

	if got := ids(r); !reflect.DeepEqual(got, []string{"A", "b", "c"}) {
		t.Errorf("order = %v", got)
	}
	// and a missing lookup is the same as no data
	r = Rank([]Candidate{acct("b"), acct("a")}, Options{Cwd: "/x", Now: now})
	if got := ids(r); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("without a lookup: %v", got)
	}
}

func TestEqualRoomGoesByLabelThenId(t *testing.T) {
	a2, a1, z := acct("2"), acct("1"), acct("b")
	a2.Label, a1.Label, z.Label = "same", "same", "zzz"
	r := Rank([]Candidate{z, a2, a1}, options("/x", map[string]Limits{
		"1": seen(20, 0), "2": seen(20, 0), "b": seen(20, 0),
	}))

	if got := ids(r); !reflect.DeepEqual(got, []string{"1", "2", "b"}) {
		t.Errorf("order = %v, want the two labeled same by id, then the one labeled zzz", got)
	}
}

func TestOldDataWhoseWindowHasResetStillCountsForItsRoom(t *testing.T) {
	// observed ten hours ago at 95% of the five-hour window, which reset since
	stale := Limits{
		FiveHour: win(95, -5*time.Hour), SevenDay: win(20, 72*time.Hour),
		ObservedAt: now.Add(-10 * time.Hour),
	}

	r := Rank([]Candidate{acct("a"), acct("b")}, options("/x", map[string]Limits{
		"a": seen(30, 30), "b": stale,
	}))

	if got := ids(r); !reflect.DeepEqual(got, []string{"b", "a"}) || r.Ranked[0].Room != 80 {
		t.Errorf("order = %v, room of b = %v, want b first with 80", got, r.Ranked[0].Room)
	}
}

func TestFolderRulesDecideWhoIsAllowedHere(t *testing.T) {
	root := t.TempDir()
	work, play := filepath.Join(root, "work"), filepath.Join(root, "play")
	for _, d := range []string{work, play} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w, p, any := acct("w"), acct("p"), acct("any")
	w.Folders, p.Folders = []string{work}, []string{play}

	r := Rank([]Candidate{w, p, any}, options(filepath.Join(work, "app"), nil))

	if got := ids(r); !reflect.DeepEqual(got, []string{"any", "w"}) {
		t.Errorf("allowed = %v, want any and w", got)
	}
	if len(r.Left) != 1 || r.Left[0].ID != "p" || !strings.Contains(r.Left[0].Why, "folder rules") ||
		!strings.Contains(r.Left[0].Why, filepath.Join(work, "app")) {
		t.Errorf("left = %+v", r.Left)
	}
}

func TestWhoIsLeftOutAndWhy(t *testing.T) {
	gone, out, switchOff, current := acct("gone"), acct("out"), acct("manual"), acct("here")
	gone.Hidden, out.SignedIn = true, false

	r := Rank([]Candidate{gone, out, switchOff, current, acct("ok")}, Options{
		Cwd: "/x", Now: now, Exclude: "here", NeedAutoSwitch: true,
	})
	// nobody here has auto-switch on, so everyone who is not out for another
	// reason is out for that one
	why := map[string]string{}
	for _, l := range r.Left {
		why[l.ID] = l.Why
	}
	want := map[string]string{
		"gone":   "was removed",
		"out":    "is not signed in",
		"manual": "has auto-switch off",
		"here":   "is the account the session is in",
		"ok":     "has auto-switch off",
	}
	if !reflect.DeepEqual(why, want) || len(r.Ranked) != 0 {
		t.Errorf("left = %v, ranked = %v, want left %v", why, ids(r), want)
	}
}

func TestAutoSwitchOnlyLooksAtAccountsThatHaveItOn(t *testing.T) {
	on, off := acct("on"), acct("off")
	on.AutoSwitch = true

	r := Rank([]Candidate{off, on}, Options{Cwd: "/x", Now: now, NeedAutoSwitch: true})
	if got := ids(r); !reflect.DeepEqual(got, []string{"on"}) {
		t.Errorf("ranked = %v", got)
	}
	// and the flag does not matter when it is not asked for
	r = Rank([]Candidate{off, on}, Options{Cwd: "/x", Now: now})
	if got := ids(r); !reflect.DeepEqual(got, []string{"off", "on"}) {
		t.Errorf("ranked = %v", got)
	}
}

func TestEveryoneLeftOutIsAnEmptyResultWithReasons(t *testing.T) {
	out := acct("out")
	out.SignedIn = false

	r := Rank([]Candidate{out}, options("/x", nil))

	if len(r.Ranked) != 0 || len(r.Left) != 1 {
		t.Errorf("result = %+v", r)
	}
	if r := Rank(nil, options("/x", nil)); len(r.Ranked) != 0 || len(r.Left) != 0 {
		t.Errorf("result for no accounts = %+v", r)
	}
}

func TestTheReasonSaysWhyTheFirstOneWon(t *testing.T) {
	cases := []struct {
		name  string
		cands []Candidate
		by    map[string]Limits
		want  []string
	}{
		{"most room with both windows", []Candidate{acct("a"), acct("b")},
			map[string]Limits{"a": seen(50, 10), "b": seen(12, 30)},
			[]string{"most room", "5-hour window 12% used", "7-day window 30% used"}},
		{"most room with one window", []Candidate{acct("a"), acct("b")},
			map[string]Limits{"a": {FiveHour: win(50, time.Hour), ObservedAt: now}, "b": {FiveHour: win(12, time.Hour), ObservedAt: now}},
			[]string{"most room", "5-hour window 12% used"}},
		{"no data", []Candidate{acct("a"), acct("b")}, nil,
			[]string{"no recent limit data", "first by label"}},
		{"the only one", []Candidate{acct("a")}, map[string]Limits{"a": seen(12, 30)},
			[]string{"the only allowed account"}},
	}
	for _, c := range cases {
		r := Rank(c.cands, options("/x", c.by))
		for _, want := range c.want {
			if !strings.Contains(r.Ranked[0].Reason, want) {
				t.Errorf("%s: reason %q lacks %q", c.name, r.Ranked[0].Reason, want)
			}
		}
	}
	// the seven-day window is not mentioned when it is not known
	r := Rank([]Candidate{acct("a"), acct("b")}, options("/x", map[string]Limits{
		"a": {FiveHour: win(50, time.Hour), ObservedAt: now}, "b": {FiveHour: win(12, time.Hour), ObservedAt: now},
	}))
	if strings.Contains(r.Ranked[0].Reason, "7-day") {
		t.Errorf("reason = %q", r.Ranked[0].Reason)
	}
}

func TestAMaxAgeOfOnesOwnIsUsed(t *testing.T) {
	l := seen(10, 10)
	l.ObservedAt = now.Add(-2 * time.Hour)
	o := options("/x", map[string]Limits{"b": l})

	if r := Rank([]Candidate{acct("a"), acct("b")}, o); !reflect.DeepEqual(ids(r), []string{"b", "a"}) {
		t.Errorf("with the default max age: %v", ids(r))
	}
	o.MaxAge = time.Hour
	if r := Rank([]Candidate{acct("a"), acct("b")}, o); !reflect.DeepEqual(ids(r), []string{"a", "b"}) {
		t.Errorf("with an hour: %v", ids(r))
	}
}
