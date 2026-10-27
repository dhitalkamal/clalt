package cli

import (
	"strings"
	"testing"
)

// usage reports file the usage that no account can claim under unattributed. an
// account with that label would be taken for it, so no account can have the
// label, in any case, which is how labels are compared everywhere else
var reservedSpellings = []string{"unattributed", "Unattributed", "UNATTRIBUTED", "uNaTtRiBuTeD"}

func wantReservedRefusal(t *testing.T, e *testEnv, code int, label string) {
	t.Helper()
	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "reserved") || !strings.Contains(msg, `"`+label+`"`) || !strings.Contains(msg, "choose another label") {
		t.Errorf("exit %d, want 1 with a message that says %q is reserved and to choose another label:\n%s", code, label, msg)
	}
}

func TestLoginRefusesTheReservedLabelInAnyCase(t *testing.T) {
	for _, label := range reservedSpellings {
		t.Run(label, func(t *testing.T) {
			e := newEnv(t)
			e.claude.signInAs("a@example.com")

			code := e.run("login", label)

			wantReservedRefusal(t, e, code, label)
			if accts, err := e.app.Store.List(); err != nil || len(accts) != 0 {
				t.Errorf("an account was made: %v, %v", accts, err)
			}
			noClaudeCalls(t, e)
			noFolderOutside(t, e)
		})
	}
}

func TestRenameRefusesTheReservedLabelInAnyCase(t *testing.T) {
	for _, label := range reservedSpellings {
		t.Run(label, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")

			code := e.run("rename", "work", label)

			wantReservedRefusal(t, e, code, label)
			if got := e.account("work").Label(); got != "work" {
				t.Errorf("the label is now %q", got)
			}
		})
	}
}

// every rename goes through the same call, so no command gets round the rule
func TestRelabelRefusesTheReservedLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	err := e.app.relabel(e.account("work").Name, "Unattributed")

	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Errorf("err = %v, want the label to be refused as reserved", err)
	}
	if got := e.account("work").Label(); got != "work" {
		t.Errorf("the label is now %q", got)
	}
}

// only the name itself is reserved
func TestLabelsThatOnlyContainTheReservedNameAreFine(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	for _, label := range []string{"unattributed-2", "not-unattributed", "unattributed.work", "unattribute"} {
		if code := e.run("rename", "work", label); code != 0 {
			t.Fatalf("rename to %q exited %d: %s", label, code, e.errOut.String())
		}
		if got := e.account(label).Label(); got != label {
			t.Errorf("the label is %q, want %q", got, label)
		}
		if code := e.run("rename", label, "work"); code != 0 {
			t.Fatalf("rename back from %q exited %d: %s", label, code, e.errOut.String())
		}
	}
	e.claude.signInAs("other@example.com")
	if code := e.run("login", "unattributed-3"); code != 0 {
		t.Errorf("login unattributed-3 exited %d: %s", code, e.errOut.String())
	}
}

// v0.1.0 let an account have the label. it can be renamed away, since only the
// new label is checked, and it cannot be signed in under the label again
func TestAnAccountThatHasTheReservedLabelFromV010CanBeRenamedAway(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{"unattributed":{"email":"u@example.com","addedAt":"2026-01-01T00:00:00Z"}}}`)

	if code := e.run("login", "unattributed"); code != 1 || !strings.Contains(e.errOut.String(), "reserved") {
		t.Errorf("login exited %d: %s", code, e.errOut.String())
	}
	noClaudeCalls(t, e)
	e.reset()

	if code := e.run("rename", "unattributed", "work"); code != 0 {
		t.Fatalf("rename exited %d: %s", code, e.errOut.String())
	}
	if got := e.account("work").Name; got != "unattributed" {
		t.Errorf("the account kept its id: got %q", got)
	}
}
