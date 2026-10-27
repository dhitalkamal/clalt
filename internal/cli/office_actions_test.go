package cli

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/runcmd"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

func refusedReason(t *testing.T, err error) string {
	t.Helper()
	var r *office.RefusedError
	if !errors.As(err, &r) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	return r.Reason
}

func TestOfficeMoveLeavesTheSameRequestAsClaltMove(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")
	to := e.account("personal").Name

	if err := e.app.officeActions().Move(theHandle, to); err != nil {
		t.Fatal(err)
	}
	if req, ok := requestOf(t, reg); !ok || req.To != to {
		t.Errorf("request = %+v, %v", req, ok)
	}
}

func TestOfficeMoveRefusesWhatClaltMoveRefuses(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")
	acts := e.app.officeActions()
	personal := e.account("personal").Name

	cases := map[string]struct{ handle, to, want string }{
		"already there":            {theHandle, e.account("work").Name, "already running as work"},
		"no such account":          {theHandle, "nobody", "nobody"},
		"no such session":          {"ffffffffffffffff", personal, "no running session"},
		"a prefix is not a handle": {theHandle[:8], personal, "no running session"},
	}
	for name, c := range cases {
		if got := refusedReason(t, acts.Move(c.handle, c.to)); !strings.Contains(got, c.want) {
			t.Errorf("%s: reason %q lacks %q", name, got, c.want)
		}
	}
	if _, ok := requestOf(t, reg); ok {
		t.Error("a refused move left a request")
	}
}

func TestOfficeMoveRefusesAnEndedSessionAndAnUnsupportedSystem(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")
	openFeed = func() wrapper.Feed { return moveFeed{theHandle: {State: wrapper.Ended}} }
	if got := refusedReason(t, e.app.officeActions().Move(theHandle, e.account("personal").Name)); !strings.Contains(got, "ended") {
		t.Errorf("ended: %q", got)
	}
	openFeed = func() wrapper.Feed { return moveFeed{theHandle: {State: wrapper.Idle}} }
	moveSupported = false
	if got := refusedReason(t, e.app.officeActions().Move(theHandle, e.account("personal").Name)); !strings.Contains(got, "windows") {
		t.Errorf("unsupported: %q", got)
	}
	if _, ok := requestOf(t, reg); ok {
		t.Error("a request was left")
	}
}

func TestOfficeFocusUsesTheRecordedLocation(t *testing.T) {
	e, f := jumpEnv(t)

	if err := e.app.officeActions().Focus(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	want := []runcmd.Call{{Name: "kitten", Args: []string{"@", "--to", "unix:/tmp/k", "focus-window", "--match", "id:7"}}}
	if !reflect.DeepEqual(f.run.Calls(), want) {
		t.Errorf("calls = %v", f.run.Calls())
	}
}

func TestOfficeFocusRefusesWhatCannotBeFocused(t *testing.T) {
	e, f := jumpEnv(t)
	acts := e.app.officeActions()
	for _, c := range []struct{ handle, want string }{
		{"gamma", "vscode"}, {"delta", "plain terminal"}, {"nobody", "no running session"}, {"alp", "no running session"},
	} {
		if got := refusedReason(t, acts.Focus(context.Background(), c.handle)); !strings.Contains(got, c.want) {
			t.Errorf("%s: reason %q lacks %q", c.handle, got, c.want)
		}
	}
	if len(f.run.Calls()) != 0 {
		t.Errorf("ran %v for a session that cannot be focused", f.run.Calls())
	}
}

func TestOfficeStopLeavesAStopRequestForTheWrapperOnly(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")

	if err := e.app.officeActions().Stop(theHandle); err != nil {
		t.Fatal(err)
	}
	if !wrapper.HasStop(reg.dir, theHandle) {
		t.Error("no stop request")
	}
}

func TestOfficeStopRefusesAnUnknownEndedOrUnsupportedSession(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")
	acts := e.app.officeActions()
	if got := refusedReason(t, acts.Stop("ffffffffffffffff")); !strings.Contains(got, "no running session") {
		t.Errorf("unknown: %q", got)
	}
	if got := refusedReason(t, acts.Stop(theHandle[:8])); !strings.Contains(got, "no running session") {
		t.Errorf("prefix: %q", got)
	}
	openFeed = func() wrapper.Feed { return moveFeed{theHandle: {State: wrapper.Ended}} }
	if got := refusedReason(t, acts.Stop(theHandle)); !strings.Contains(got, "ended") {
		t.Errorf("ended: %q", got)
	}
	openFeed = func() wrapper.Feed { return moveFeed{theHandle: {State: wrapper.Idle}} }
	moveSupported = false
	if got := refusedReason(t, acts.Stop(theHandle)); !strings.Contains(got, "windows") {
		t.Errorf("unsupported: %q", got)
	}
	if wrapper.HasStop(reg.dir, theHandle) {
		t.Error("a refused stop left a request")
	}
}
