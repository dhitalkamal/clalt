package crew

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestAGatewayThatListensBeyondLoopbackIsStoppedAndReported(t *testing.T) {
	for _, out := range []string{"p1\nn*:5500\n", "p1\nn127.0.0.1:5500\nn192.168.1.5:5500\n", "p1\nn[::]:5500\n", "p1\nn0.0.0.0:5500\n"} {
		w := newWorld(t)
		w.lsof = out
		_, err := start(t, w, "work")
		if err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("%q: err = %v", out, err)
		}
		if !slices.Equal(w.signals, []int{4242}) {
			t.Errorf("%q: signals = %v, the gateway was not stopped", out, w.signals)
		}
		if _, found, _ := ReadRecord("work"); found {
			t.Errorf("%q: the record of a stopped gateway is left", out)
		}
	}
}

func TestLoopbackListenersOfBothFamiliesPass(t *testing.T) {
	w := newWorld(t)
	w.lsof = "p1\nn127.0.0.1:5500\nn[::1]:5500\n"
	if res, err := start(t, w, "work"); err != nil || res.Warning != "" {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestWhenTheBindCannotBeCheckedTheGatewayStaysAndTheResultWarns(t *testing.T) {
	for _, tc := range []struct {
		lsof string
		err  error
	}{{"", errors.New("lsof: not found")}, {"p1\n", nil}} {
		w := newWorld(t)
		w.lsof, w.lsofErr = tc.lsof, tc.err
		if tc.err == nil {
			w.lsof = "p1\n"
		}
		res, err := start(t, w, "work")
		if err != nil || !strings.Contains(res.Warning, "listens") || len(w.signals) != 0 {
			t.Errorf("res = %+v, err = %v, signals = %v", res, err, w.signals)
		}
	}
}

func TestANewerSeriesCanBeStartedOnlyWhenAskedFor(t *testing.T) {
	w := newWorld(t)
	w.version = "kirocrew 0.10.0\n"
	m := w.manager()
	m.o.AllowUntested = true
	if res, err := m.Start(context.Background(), StartRequest{Account: "work", Folder: w.folder("work")}); err != nil || res.Record.Version != "0.10.0" {
		t.Errorf("res = %+v, err = %v", res, err)
	}
	w2 := newWorld(t)
	w2.version = "garbage\n"
	m = w2.manager()
	m.o.AllowUntested = true
	if _, err := m.Start(context.Background(), StartRequest{Account: "work", Folder: w2.folder("work")}); err == nil {
		t.Error("a version that cannot be read was started")
	}
	t.Setenv(AllowUntestedVar, "1")
	if !NewManager(Options{}).o.AllowUntested {
		t.Error("the variable is not read")
	}
}

func TestTheGatewayIsToldWhereClaudeIs(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	if got := valueOf(t, w.started[0].Env, "CLAUDE_CODE_EXECUTABLE"); got != "/fake/claude" {
		t.Errorf("CLAUDE_CODE_EXECUTABLE = %q", got)
	}
	if got := valueOf(t, w.started[0].Env, "KIROCREW_BIND"); got != "127.0.0.1" {
		t.Errorf("KIROCREW_BIND = %q", got)
	}
}

func TestAStartThatFindsTheGatewayRunningWaitsForItToAnswerToo(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	asked := 0
	w.ready = func(int) bool { asked++; return asked >= 3 }
	res, err := start(t, w, "work")
	if err != nil || !res.Already || res.Warning != "" || asked < 3 {
		t.Errorf("res = %+v, err = %v, asked = %d", res, err, asked)
	}
	w.ready = func(int) bool { return false }
	res, _ = start(t, w, "work")
	if !res.Already || res.Warning == "" {
		t.Errorf("a gateway that does not answer is not a warning: %+v", res)
	}
}
