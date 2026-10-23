package cli

import (
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/tui"
)

func TestDashboardDataShowsLabelsAndTheSignInFromTheStateFile(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	e.run("logout", "home")
	e.signIn("gone", "g@example.com")
	e.run("remove", "gone", "--force")
	if err := e.app.Store.Relabel("work", "office"); err != nil {
		t.Fatal(err)
	}
	liveSession(t, "work", "/code/clalt")

	data, err := e.app.Data()

	if err != nil {
		t.Fatal(err)
	}
	if len(data.Accounts) != 2 {
		t.Fatalf("accounts = %+v, want home and office", data.Accounts)
	}
	home, office := data.Accounts[0], data.Accounts[1]
	if home.Name != "home" || home.SignedIn || !home.LoginDue.IsZero() {
		t.Errorf("home = %+v: signed out, nothing due", home)
	}
	if office.Name != "office" || !office.SignedIn || !office.LoginDue.Equal(loginDue(e.account("office").Meta.SignedInAt)) || office.Email != "w@example.com" || office.Plan != "max" {
		t.Errorf("office = %+v", office)
	}
	if len(data.Sessions) != 1 || data.Sessions[0].Account != "office" || data.Sessions[0].Cwd != "/code/clalt" {
		t.Errorf("sessions = %+v, want the session under its label", data.Sessions)
	}
}

func TestDashboardDataCarriesBudgetsAndUsage(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.run("budget", "work", "--daily", "10", "--weekly", "50")
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(replyLine)

	data, err := e.app.Data()

	if err != nil {
		t.Fatal(err)
	}
	a := data.Accounts[0]
	if a.DailyBudget != 10 || a.Stats.Billable != 300 {
		t.Errorf("account = %+v", a)
	}
}

func TestDashboardFrameForAPipeHasNoKeepaliveOrTokenPanels(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	frame, err := e.app.dashboardFrame()

	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Accounts", "Sessions", "work", "signed in"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the frame is missing %q:\n%s", want, frame)
		}
	}
	for _, gone := range []string{"Keepalive", "keepalive", "refresh"} {
		if strings.Contains(frame, gone) {
			t.Errorf("the frame still mentions %q", gone)
		}
	}
}

func TestDashboardFrameWithNoAccountsIsEmpty(t *testing.T) {
	e := newEnv(t)
	frame, err := e.app.dashboardFrame()
	if err != nil || frame != "" {
		t.Errorf("frame = %q, err = %v", frame, err)
	}
}

// the dashboard hands labels to the controller, which turns them into accounts
var _ tui.Controller = (*App)(nil)

func TestControllerRenameChangesTheLabelOnly(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if err := e.app.Rename("work", "office"); err != nil {
		t.Fatal(err)
	}

	if acc := e.account("office"); acc.Name != "work" {
		t.Errorf("id = %q", acc.Name)
	}
	if err := e.app.Rename("nobody", "x"); err == nil {
		t.Error("renaming nobody should fail")
	}
}

func TestControllerSetBudgetTakesALabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if err := e.app.SetBudget("WORK", 3, 7); err != nil {
		t.Fatal(err)
	}

	if m := e.account("work").Meta; m.DailyBudget != 3 || m.WeeklyBudget != 7 {
		t.Errorf("meta = %+v", m)
	}
	if err := e.app.SetBudget("nobody", 1, 1); err == nil {
		t.Error("a budget for nobody should fail")
	}
}

func TestControllerLogoutSignsOutAndSaysNothingOnTheTerminal(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if err := e.app.Logout("work"); err != nil {
		t.Fatal(err)
	}

	if e.claude.signedIn(e.folder("work")) || !e.account("work").Meta.SignedInAt.IsZero() {
		t.Error("the account is still signed in")
	}
	// the dashboard owns the screen while this runs
	if e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("printed %q and %q", e.out.String(), e.errOut.String())
	}
}

func TestControllerRemoveSignsOutThenHides(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if err := e.app.Remove("work"); err != nil {
		t.Fatal(err)
	}

	if !e.account("work").Meta.Hidden || e.claude.signedIn(e.folder("work")) {
		t.Error("the account was not signed out and hidden")
	}
	if e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("printed %q and %q", e.out.String(), e.errOut.String())
	}
}

func TestControllerRemoveThatCannotSignOutKeepsTheAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_FAILS", "1")

	err := e.app.Remove("work")

	if err == nil || !strings.Contains(err.Error(), "could not reach the keychain") {
		t.Errorf("err = %v", err)
	}
	if e.account("work").Meta.Hidden {
		t.Error("the account was hidden")
	}
}

func TestControllerRemoveOfARemovedAccountFails(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := e.app.Remove("work"); err != nil {
		t.Fatal(err)
	}

	if err := e.app.Remove("work"); err == nil {
		t.Error("removing twice should fail")
	}
}
