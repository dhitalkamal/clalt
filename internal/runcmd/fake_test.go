package runcmd

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestFakeRecordsEveryCallInOrder(t *testing.T) {
	f := NewFake()

	f.Run(context.Background(), "tmux", "new-window", "-P")
	f.Run(context.Background(), "kitten", "@", "ls")

	want := []Call{{Name: "tmux", Args: []string{"new-window", "-P"}}, {Name: "kitten", Args: []string{"@", "ls"}}}
	if got := f.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("Calls() = %v, want %v", got, want)
	}
}

func TestFakeKeepsACopyOfTheArguments(t *testing.T) {
	f := NewFake()
	args := []string{"one", "two"}

	f.Run(context.Background(), "x", args...)
	args[0] = "changed"

	if got := f.Calls()[0].Args[0]; got != "one" {
		t.Errorf("the recorded argument became %q", got)
	}
}

func TestFakeAnswersFromTheFirstRuleThatMatches(t *testing.T) {
	f := NewFake()
	f.On("tmux", "new-window").Output("main:3")
	f.On("tmux").Output("anything else")

	first, _ := f.Run(context.Background(), "tmux", "new-window", "-P")
	second, _ := f.Run(context.Background(), "tmux", "list-panes")

	if string(first) != "main:3" || string(second) != "anything else" {
		t.Errorf("answers %q and %q", first, second)
	}
}

func TestFakeMatchesOnTheStartOfTheArguments(t *testing.T) {
	f := NewFake()
	f.On("kitten", "@", "launch").Output("7")

	hit, _ := f.Run(context.Background(), "kitten", "@", "launch", "--type=tab")
	miss, _ := f.Run(context.Background(), "kitten", "@", "ls")
	other, _ := f.Run(context.Background(), "tmux", "@", "launch")

	if string(hit) != "7" || len(miss) != 0 || len(other) != 0 {
		t.Errorf("hit %q, miss %q, other program %q", hit, miss, other)
	}
}

func TestFakeFailsTheWayExecDoes(t *testing.T) {
	f := NewFake()
	f.On("tmux", "switch-client").Fail(1, "no current client\nsecond line")

	_, err := f.Run(context.Background(), "tmux", "switch-client", "-t", "%5")

	var fail *Failure
	if !errors.As(err, &fail) {
		t.Fatalf("err = %v, want a *Failure", err)
	}
	if fail.Name != "tmux" || fail.Code != 1 || fail.Stderr != "no current client\nsecond line" {
		t.Errorf("Failure = %+v", fail)
	}
}

func TestFakeCanFailToRunAtAll(t *testing.T) {
	f := NewFake()
	boom := errors.New("no such program")
	f.On("osascript").Err(boom)

	if _, err := f.Run(context.Background(), "osascript", "-e", "x"); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}

func TestFakeSucceedsWithNoOutputWhenNoRuleMatches(t *testing.T) {
	out, err := NewFake().Run(context.Background(), "anything")

	if err != nil || len(out) != 0 {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestFakeHasIsTrueUnlessLeftOut(t *testing.T) {
	f := NewFake().Without("kitten", "notify-send")

	if f.Has("kitten") || f.Has("notify-send") || !f.Has("tmux") {
		t.Errorf("Has: kitten %v, notify-send %v, tmux %v", f.Has("kitten"), f.Has("notify-send"), f.Has("tmux"))
	}
}

func TestFakeIsSafeToUseFromSeveralGoroutines(t *testing.T) {
	f := NewFake()
	f.On("x").Output("ok")
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.Run(context.Background(), "x", "a")
			f.Has("x")
			f.Calls()
		}()
	}
	wg.Wait()

	if n := len(f.Calls()); n != 20 {
		t.Errorf("%d calls recorded, want 20", n)
	}
}
