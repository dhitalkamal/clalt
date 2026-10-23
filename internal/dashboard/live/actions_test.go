package live

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/dashboard"
)

var bg = context.Background()

func TestRenameChangesTheLabelByIdAndNothingElse(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	if err := r.adapter().Rename(bg, id, "office"); err != nil {
		t.Fatal(err)
	}
	acc, found, _ := r.store.ResolveLabel("office")
	if !found || acc.Name != id {
		t.Errorf("found %v, id %q", found, acc.Name)
	}
	if err := r.adapter().Rename(bg, "nobody", "x"); err == nil {
		t.Error("renaming nobody fails")
	}
	if err := r.adapter().Rename(bg, id, "bad name"); err == nil {
		t.Error("a label the store refuses is an error")
	}
}

func TestBudgetSetsBothCapsAndRefusesNonsense(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	a := r.adapter()
	if err := a.SetBudget(bg, id, 3, 7); err != nil {
		t.Fatal(err)
	}
	if m := r.store; m == nil {
		t.Fatal()
	}
	acc, _, _ := r.store.ResolveLabel("work")
	if acc.Meta.DailyBudget != 3 || acc.Meta.WeeklyBudget != 7 {
		t.Errorf("meta = %+v", acc.Meta)
	}
	if err := a.SetBudget(bg, id, 0, 0); err != nil {
		t.Fatal(err)
	}
	acc, _, _ = r.store.ResolveLabel("work")
	if acc.Meta.DailyBudget != 0 || acc.Meta.WeeklyBudget != 0 {
		t.Errorf("0 clears the caps: %+v", acc.Meta)
	}
	for _, bad := range [][2]float64{{-1, 0}, {0, math.NaN()}, {math.Inf(1), 0}} {
		if err := a.SetBudget(bg, id, bad[0], bad[1]); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	acc, _, _ = r.store.ResolveLabel("work")
	if acc.Meta.DailyBudget != 0 {
		t.Error("a refused amount changes nothing")
	}
}

func TestSignOutCallsTheHostAndTheNextLoadShowsIt(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	a := r.adapter()
	if !byLabel(r.load(a), "work").SignedIn {
		t.Fatal("setup: signed in")
	}

	if err := a.SignOut(bg, id); err != nil {
		t.Fatal(err)
	}

	if len(r.host.signOuts) != 1 || r.host.signOuts[0] != id {
		t.Errorf("host sign-outs = %v", r.host.signOuts)
	}
	if acc := byLabel(r.load(a), "work"); acc.SignedIn {
		t.Errorf("still signed in after a sign-out: %+v", acc)
	}
}

func TestASignOutThatFailsChangesWhatIsShownNotAtAll(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	a := r.adapter()
	r.load(a)
	r.host.signErr = errors.New("could not reach the keychain")

	err := a.SignOut(bg, id)

	if err == nil || !strings.Contains(err.Error(), "keychain") {
		t.Errorf("err = %v", err)
	}
	if !byLabel(r.load(a), "work").SignedIn {
		t.Error("the account is still signed in")
	}
}

func TestRunAndSignInLaunchTheHostsCodeAndForgetWhatWasKnown(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	a := r.adapter()
	r.load(a)
	checks, scans := r.host.checks.Load(), r.host.scans.Load()

	run, err := a.Run(bg, id)
	if err != nil || run == nil {
		t.Fatalf("Run = %v, %v", run, err)
	}
	login, _ := a.SignIn(bg, id)
	if got := strings.Join(r.host.launches, ","); got != "run work,login work" {
		t.Errorf("host launches = %s", got)
	}
	r.host.signIns[id] = SignIn{Reason: claudecli.ReasonNotSignedIn}
	if err := run.Run(); err != nil {
		t.Fatal(err)
	}
	_ = login

	if acc := byLabel(r.load(a), "work"); acc.SignedIn {
		t.Errorf("after the session the login is asked about again: %+v", acc)
	}
	if r.host.checks.Load() == checks || r.host.scans.Load() == scans {
		t.Errorf("checks %d to %d, scans %d to %d", checks, r.host.checks.Load(), scans, r.host.scans.Load())
	}
}

func TestALaunchKeepsTheErrorOfWhatItRan(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	l, _ := r.adapter().Run(bg, id)
	l.(launched).Launch.(*runLaunch).err = errors.New("refused")
	if err := l.Run(); err == nil || err.Error() != "refused" {
		t.Errorf("err = %v", err)
	}
}

func TestRunRefusesAnUnknownAccountAndALabelThatLooksLikeAFlag(t *testing.T) {
	r := newRig(t)
	if _, err := r.adapter().Run(bg, "nobody"); err == nil {
		t.Error("an unknown id is an error")
	}
	writeState(t, `{"accounts":{"odd":{"label":"-x","email":"o@example.com","addedAt":"2026-01-01T00:00:00Z"}}}`)
	if _, err := r.adapter().Run(bg, "odd"); err == nil || !strings.Contains(err.Error(), "dash") {
		t.Errorf("err = %v", err)
	}
	if len(r.host.launches) != 0 {
		t.Errorf("nothing was launched: %v", r.host.launches)
	}
}

func TestStopGoesToTheRegistryAndTheDefaultSaysItCannot(t *testing.T) {
	r := newRig(t)
	if err := r.adapter().StopSession(bg, "work", "h1"); err == nil {
		t.Error("without a registry there is nothing to stop")
	} else if _, ok := err.(dashboard.Notice); !ok {
		t.Errorf("that is a message, not a failure: %T", err)
	}
	s := &fakeSessions{}
	r.cfg.Sessions = s
	if err := r.adapter().StopSession(bg, "work", "h1"); err != nil || len(s.stopped) != 1 || s.stopped[0] != "work/h1" {
		t.Errorf("stopped = %v, %v", s.stopped, err)
	}
}

func TestRenameAndBudgetRefuseAHiddenAccount(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("gone", "g@example.com")
	r.store.Hide(id)
	a := r.adapter()
	if err := a.Rename(bg, id, "back"); err == nil {
		t.Error("a removed account is not renamed from the dashboard")
	}
	if err := a.SetBudget(bg, id, 1, 1); err == nil {
		t.Error("a removed account gets no budget from the dashboard")
	}
	if acc, _, _ := r.store.ResolveLabel("gone"); acc.Meta.DailyBudget != 0 {
		t.Errorf("meta = %+v", acc.Meta)
	}
}
