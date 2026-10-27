package demo

import (
	"context"
	"errors"
	"testing"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

func TestLoadReturnsAFreshSnapshotEveryTime(t *testing.T) {
	src := Source(atEpoch)
	first, err := src.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first.Accounts[0].Label = "changed"
	first.Accounts[0].History.Days[0].Cost = 99
	first.Accounts[0].Sessions[0].Folder = "changed"
	second, _ := src.Load(context.Background())
	if second.Accounts[0].Label != "robin" || second.Accounts[0].History.Days[0].Cost == 99 || second.Accounts[0].Sessions[0].Folder == "changed" {
		t.Error("a caller changing one snapshot must not change the next one")
	}
}

func TestLoadStopsForACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Source(atEpoch).Load(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Load on a cancelled context = %v", err)
	}
}

func TestEveryDemoActionJustSaysWhatItWouldDo(t *testing.T) {
	act := Actions()
	ctx := context.Background()
	notice := func(err error) string {
		n, ok := err.(dashboard.Notice)
		if !ok {
			t.Fatalf("expected a Notice, got %v", err)
		}
		return string(n)
	}
	cmd, err := act.Run(ctx, "robin")
	if cmd != nil || notice(err) != "Demo: would start claude as robin." {
		t.Errorf("Run = %v, %v", cmd, err)
	}
	cmd, err = act.SignIn(ctx, "harriet")
	if cmd != nil || notice(err) != "Demo: would sign in to harriet." {
		t.Errorf("SignIn = %v, %v", cmd, err)
	}
	cases := []struct {
		err  error
		want string
	}{
		{act.Rename(ctx, "robin", "work"), "Demo: would rename robin to work."},
		{act.SetBudget(ctx, "robin", 12.5, 0), "Demo: would set robin's budget to $12.50 a day and no weekly cap."},
		{act.SetBudget(ctx, "casey", 8, 40), "Demo: would set casey's budget to $8.00 a day and $40.00 a week."},
		{act.SetBudget(ctx, "morgan", 0, 0), "Demo: would clear morgan's budget."},
		{act.SignOut(ctx, "jordan"), "Demo: would sign out jordan (history kept)."},
		{act.Delete(ctx, "jordan"), "Demo: would delete jordan and its folder (history stays)."},
		{act.StopSession(ctx, "robin", "k-live-1"), "Demo: would stop that session."},
	}
	for _, c := range cases {
		if got := notice(c.err); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
