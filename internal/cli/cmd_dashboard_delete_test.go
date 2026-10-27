package cli

import (
	"strings"
	"testing"
)

func TestTheHostDeletesLikeRemoveDeleteForce(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	folder := e.folder("work")

	if err := dashHost(e).Delete(e.account("work")); err != nil {
		t.Fatal(err)
	}

	if exists(folder) || e.hasAccount("work") {
		t.Error("the folder or the record is still there")
	}
	if len(e.claude.calls("logout")) == 0 {
		t.Error("the account was not signed out first")
	}
	if e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("the dashboard owns the screen: %q %q", e.out.String(), e.errOut.String())
	}
}

func TestTheHostDeleteKeepsEverythingWhenTheSignOutFails(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	folder := e.folder("work")
	e.claude.set("LOGOUT_FAILS", "1")

	err := dashHost(e).Delete(e.account("work"))

	if err == nil || !strings.Contains(err.Error(), "could not sign") {
		t.Errorf("err = %v", err)
	}
	if !exists(folder) || !e.hasAccount("work") {
		t.Error("something was deleted after a failed sign-out")
	}
}
