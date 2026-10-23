package defaulthome

import (
	"errors"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/claudecli"
)

var now = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

func signedIn(email string) claudecli.AuthStatus {
	return claudecli.AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", APIProvider: "firstParty", Email: email, ConfigDirectory: "/h/.claude"}
}

func TestDecide(t *testing.T) {
	accts := []Candidate{{ID: "work", Email: "Me@Example.com"}, {ID: "side", Email: "side@example.com"}}
	api := signedIn("me@example.com")
	api.AuthMethod = "api_key_helper"
	third := signedIn("me@example.com")
	third.APIProvider = "bedrock"
	cases := []struct {
		name    string
		st      claudecli.AuthStatus
		err     error
		accts   []Candidate
		state   State
		account string
	}{
		{"match", signedIn("me@example.com"), nil, accts, Matched, "work"},
		{"match ignores case", signedIn("ME@EXAMPLE.COM"), nil, accts, Matched, "work"},
		{"no account has the email", signedIn("other@example.com"), nil, accts, Outside, ""},
		{"not claude.ai", api, nil, accts, Outside, ""},
		{"third party provider", third, nil, accts, Outside, ""},
		{"signed out", claudecli.AuthStatus{}, nil, accts, Outside, ""},
		{"claude unusable", claudecli.AuthStatus{}, errors.New("claude is missing"), accts, Unusable, ""},
		{"two accounts with the email", signedIn("me@example.com"), nil,
			[]Candidate{{ID: "a", Email: "me@example.com"}, {ID: "b", Email: "ME@example.com"}}, Outside, ""},
		{"account without an email never matches", signedIn("me@example.com"), nil, []Candidate{{ID: "a"}}, Outside, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.st, c.err, "/h/.claude", c.accts, now)
			if got.State != c.state || got.Account != c.account {
				t.Errorf("got %s %q (%s), want %s %q", got.State, got.Account, got.Reason, c.state, c.account)
			}
			if c.state != Matched && got.Reason == "" {
				t.Error("no reason given")
			}
			if !got.CheckedAt.Equal(now) {
				t.Errorf("checkedAt %v", got.CheckedAt)
			}
		})
	}
}

func TestUnusableKeepsTheEarlierMatchButNotesTheFailure(t *testing.T) {
	old := Cache{State: Matched, Account: "work", Email: "me@example.com", CheckedAt: now.Add(-48 * time.Hour), Off: true}
	got := Merge(old, Decide(claudecli.AuthStatus{}, errors.New("timed out"), "/h/.claude", nil, now))
	if got.State != Matched || got.Account != "work" || !got.Off {
		t.Errorf("lost the match: %+v", got)
	}
	if got.Reason == "" || !got.CheckedAt.Equal(now) {
		t.Errorf("failure not noted: %+v", got)
	}
}

func TestAFreshAnswerReplacesTheOldOneAndKeepsOff(t *testing.T) {
	old := Cache{State: Matched, Account: "work", Off: true}
	got := Merge(old, Decide(signedIn("x@example.com"), nil, "/h/.claude", nil, now))
	if got.State != Outside || got.Account != "" || !got.Off {
		t.Errorf("got %+v", got)
	}
}

func TestCacheRoundTripAndBadFiles(t *testing.T) {
	p := t.TempDir() + "/default-home.json"
	if _, ok := Read(p); ok {
		t.Error("a missing file counted")
	}
	want := Cache{State: Matched, Account: "work", Email: "me@example.com", CheckedAt: now}
	if err := Write(p, want); err != nil {
		t.Fatal(err)
	}
	got, ok := Read(p)
	if !ok || got.Account != "work" || got.State != Matched {
		t.Errorf("got %+v %v", got, ok)
	}
	if err := Write(p, Cache{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(p); ok {
		t.Error("an empty cache counted")
	}
}

func TestMatchedOnlyWhenStateAndAccountAgree(t *testing.T) {
	if (Cache{State: Matched}).Active() || (Cache{State: Outside, Account: "w"}).Active() {
		t.Error("active without both")
	}
	if !(Cache{State: Matched, Account: "w"}).Active() {
		t.Error("not active")
	}
}

func TestMatchedSinceStartsWithTheMatchAndResetsWhenTheStateOrAccountChanges(t *testing.T) {
	t0, t1, t2 := now, now.Add(time.Hour), now.Add(2*time.Hour)
	a := Cache{State: Matched, Account: "work", CheckedAt: t0, MatchedSince: t0}

	same := Merge(a, Cache{State: Matched, Account: "work", CheckedAt: t1})
	if !same.MatchedSince.Equal(t0) {
		t.Errorf("same match moved the start: %v", same.MatchedSince)
	}
	other := Merge(same, Cache{State: Matched, Account: "side", CheckedAt: t2})
	if !other.MatchedSince.Equal(t2) {
		t.Errorf("a new account did not reset: %v", other.MatchedSince)
	}
	out := Merge(other, Cache{State: Outside, CheckedAt: t2})
	if !out.MatchedSince.IsZero() {
		t.Errorf("outside keeps a start: %v", out.MatchedSince)
	}
	back := Merge(out, Cache{State: Matched, Account: "side", CheckedAt: t2.Add(time.Hour)})
	if !back.MatchedSince.Equal(t2.Add(time.Hour)) {
		t.Errorf("outside -> matched backfilled: %v", back.MatchedSince)
	}
	first := Merge(Cache{}, Cache{State: Matched, Account: "work", CheckedAt: t1})
	if !first.MatchedSince.Equal(t1) {
		t.Errorf("first match: %v", first.MatchedSince)
	}
}

func TestSinceFallsBackToTheCheckTimeForACacheWithoutIt(t *testing.T) {
	c := Cache{State: Matched, Account: "w", CheckedAt: now}
	if !c.Since().Equal(now) {
		t.Errorf("%v", c.Since())
	}
}
