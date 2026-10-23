package cli

import (
	"errors"
	"testing"

	"github.com/dhitalkamal/clalt/internal/claudecli"
)

// one rule decides whether a login may be in a folder, for a login that did not
// finish and for remove alike: it does unless claude answered plainly that
// nobody is signed in. a login that something else masks still counts, and so
// does an answer that never came.
func TestALoginMayBeInAFolderUnlessClaudeSaysPlainlyThatNobodyIsSignedIn(t *testing.T) {
	cases := []struct {
		name string
		st   claudecli.AuthStatus
		err  error
		want bool
	}{
		{"nobody is signed in", claudecli.AuthStatus{LoggedIn: false, AuthMethod: "none"}, nil, false},
		{"a claude.ai login", claudecli.AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", Email: "a@example.com"}, nil, true},
		{"a login without an email, which expired", claudecli.AuthStatus{LoggedIn: true, AuthMethod: "claude.ai"}, nil, true},
		{"an api key helper masks it", claudecli.AuthStatus{LoggedIn: true, AuthMethod: "api_key_helper"}, nil, true},
		{"another provider masks it", claudecli.AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", APIProvider: "bedrock"}, nil, true},
		{"claude could not be asked", claudecli.AuthStatus{}, errors.New("keychain is locked"), true},
		{"claude could not be asked, whatever the status holds", claudecli.AuthStatus{LoggedIn: false}, errors.New("timed out"), true},
	}
	for _, c := range cases {
		if got := loginMayBeIn(c.st, c.err); got != c.want {
			t.Errorf("%s: loginMayBeIn = %v, want %v", c.name, got, c.want)
		}
	}
}

// the rule is the same one that signedInHere answers with a reason, so the two
// cannot drift apart: whatever it calls not signed in is the one case with no login
func TestTheRuleAgreesWithTheReasonSignedInHereGives(t *testing.T) {
	for _, st := range []claudecli.AuthStatus{
		{LoggedIn: false, AuthMethod: "none"},
		{LoggedIn: true, AuthMethod: "claude.ai", Email: "a@example.com", ConfigDirectory: "/x"},
		{LoggedIn: true, AuthMethod: "api_key_helper", ConfigDirectory: "/x"},
		{LoggedIn: true, AuthMethod: "claude.ai", APIProvider: "vertex", Email: "a@example.com", ConfigDirectory: "/x"},
		{LoggedIn: true, AuthMethod: "claude.ai", ConfigDirectory: "/x"},
		{LoggedIn: true, AuthMethod: "claude.ai", Email: "a@example.com"},
		{LoggedIn: true, AuthMethod: "claude.ai", Email: "a@example.com", ConfigDirectory: "/elsewhere"},
	} {
		_, why, _ := signedInHere(st, "/x")
		if got, want := loginMayBeIn(st, nil), why != claudecli.ReasonNotSignedIn; got != want {
			t.Errorf("%+v: loginMayBeIn = %v, but the reason is %v", st, got, why)
		}
	}
}
