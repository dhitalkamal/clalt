package crew

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func psAnswers(out string, err error) *fakeRunner {
	return &fakeRunner{answer: func(Spec) (string, string, error) { return out, "", err }}
}

func TestProbeReadsTheStartTimeAndTheCommandOfAProcess(t *testing.T) {
	f := psAnswers("Thu Oct 23 15:05:12 2026 /App/python3.12 -s -P -m kiro_crew gateway --port 5480 --no-open --no-tunnel\n", nil)
	p, err := Probe(context.Background(), f, nil, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Alive || p.Start != "Thu Oct 23 15:05:12 2026" || p.Command != "/App/python3.12 -s -P -m kiro_crew gateway --port 5480 --no-open --no-tunnel" {
		t.Errorf("Probe = %+v", p)
	}
}

func TestProbeKeepsADayWithOneDigitAsTheSameFiveWords(t *testing.T) {
	f := psAnswers("Thu Oct  3 09:05:12 2026 /bin/sh /x/kirocrew gateway --port 5480\n", nil)
	p, err := Probe(context.Background(), f, nil, 4242)
	if err != nil || p.Start != "Thu Oct 3 09:05:12 2026" || p.Command != "/bin/sh /x/kirocrew gateway --port 5480" {
		t.Errorf("Probe = %+v, %v", p, err)
	}
}

func TestProbeAsksPsForOneProcessInTheCLocaleWithTheWholeCommand(t *testing.T) {
	f := psAnswers("Thu Oct 23 15:05:12 2026 cmd\n", nil)
	if _, err := Probe(context.Background(), f, nil, 4242); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("ps was called %d times, want once", len(f.calls))
	}
	call := f.calls[0]
	if call.Bin != "ps" || !slices.Equal(call.Args, []string{"-ww", "-o", "lstart=,command=", "-p", "4242"}) {
		t.Errorf("ps was called as %s %v", call.Bin, call.Args)
	}
	if !slices.Contains(call.Env, "LC_ALL=C") {
		t.Errorf("the locale was not fixed: %v", call.Env)
	}
}

func TestAProcessThatPsDoesNotKnowIsNotAlive(t *testing.T) {
	p, err := Probe(context.Background(), psAnswers("", exitErr{1}), nil, 4242)
	if err != nil || p.Alive {
		t.Errorf("Probe = %+v, %v, want a process that is not alive", p, err)
	}
}

func TestADefunctProcessIsNotAlive(t *testing.T) {
	p, err := Probe(context.Background(), psAnswers("Thu Oct 23 15:05:12 2026 [python3.12] <defunct>\n", nil), nil, 4242)
	if err != nil || p.Alive {
		t.Errorf("Probe = %+v, %v", p, err)
	}
}

func TestWhenPsCannotBeRunTheAnswerIsAnErrorAndNotADeadProcess(t *testing.T) {
	_, err := Probe(context.Background(), psAnswers("", errors.New("executable file not found")), nil, 4242)
	if err == nil || !strings.Contains(err.Error(), "ps") {
		t.Errorf("err = %v", err)
	}
}

func TestOutputThatIsNotAStartTimeAndACommandIsAnError(t *testing.T) {
	for _, out := range []string{"garbage\n", "PID TTY TIME CMD\n", "Thu Oct 23 15:05:12 2026\n"} {
		if p, err := Probe(context.Background(), psAnswers(out, nil), nil, 4242); err == nil {
			t.Errorf("Probe(%q) = %+v, want an error", out, p)
		}
	}
}

func TestProbeRunsPsInTheEnvironmentItIsGiven(t *testing.T) {
	f := psAnswers("Thu Oct 23 15:05:12 2026 cmd\n", nil)
	if _, err := Probe(context.Background(), f, []string{"PATH=/x", "FOO=bar"}, 4242); err != nil {
		t.Fatal(err)
	}
	env := f.calls[0].Env
	if !slices.Contains(env, "FOO=bar") || !slices.Contains(env, "LC_ALL=C") {
		t.Errorf("env = %v", env)
	}
}

func TestAPidThatCannotBeAPidIsNotAsked(t *testing.T) {
	f := psAnswers("", nil)
	for _, pid := range []int{0, -1} {
		if p, err := Probe(context.Background(), f, nil, pid); err != nil || p.Alive {
			t.Errorf("Probe(%d) = %+v, %v", pid, p, err)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("ps was asked about pid 0 or below: %v", f.calls)
	}
}

func TestTheStandingOfARecordedGateway(t *testing.T) {
	rec := Record{PID: 4242, Port: 5480, ProcStart: "Thu Oct 23 15:05:12 2026"}
	python := "/App/python3.12 -s -P -m kiro_crew gateway --port 5480 --no-open --no-tunnel"
	for name, tc := range map[string]struct {
		rec  Record
		proc Proc
		want State
	}{
		"the same process still a gateway": {rec, Proc{Alive: true, Start: rec.ProcStart, Command: python}, StateRunning},
		"the fake launcher":                {rec, Proc{Alive: true, Start: rec.ProcStart, Command: "/bin/sh /t/kirocrew gateway --port 5480"}, StateRunning},
		"gone":                             {rec, Proc{}, StateStale},
		"the pid went to another process":  {rec, Proc{Alive: true, Start: "Fri Oct 24 01:00:00 2026", Command: python}, StateStale},
		"same start, another command":      {rec, Proc{Alive: true, Start: rec.ProcStart, Command: "vim notes.txt"}, StateUnverified},
		"same start, another port":         {rec, Proc{Alive: true, Start: rec.ProcStart, Command: "/App/python3.12 -m kiro_crew gateway --port 5481"}, StateUnverified},
		"no start time was kept":           {Record{PID: 4242, Port: 5480}, Proc{Alive: true, Start: rec.ProcStart, Command: python}, StateUnverified},
	} {
		got, why := Judge(tc.rec, tc.proc)
		if got != tc.want {
			t.Errorf("%s: Judge = %s (%s), want %s", name, got, why, tc.want)
		}
		if (got == StateRunning) != (why == "") {
			t.Errorf("%s: why = %q for state %s", name, why, got)
		}
	}
}

func TestOnlyAKiroCrewGatewayOnTheRightPortLooksLikeOne(t *testing.T) {
	for command, want := range map[string]bool{
		"/App/python3.12 -s -P -m kiro_crew gateway --port 5480 --no-open --no-tunnel": true,
		"/bin/sh /tmp/x/kirocrew gateway --port 5480 --no-open --no-tunnel":            true,
		"kirocrew gateway --port=5480":                                                 true,
		"KIROCREW gateway --port 5480":                                                 true,
		"/App/python3.12 -m kiro_crew gateway --port 5481":                             false,
		"/App/python3.12 -m kiro_crew gateway --port 54800":                            false,
		"/App/python3.12 -m kiro_crew status --port 5480":                              false,
		"vim gateway --port 5480":                                                      false,
		"/App/python3.12 -m kiro_crew gateway":                                         false,
		"vim kirocrew gateway --port 5480":                                             false,
		"/usr/bin/tail -f /tmp/kirocrew/gateway --port 5480":                           false,
		"/bin/sh -c echo kirocrew gateway --port 5480":                                 false,
		"/usr/bin/python3 -m kiro_crew status gateway --port 5480":                     false,
		"/usr/local/bin/kirocrew -v gateway --port 5480":                               true,
		"": false,
	} {
		if got := looksLikeGateway(command, 5480); got != want {
			t.Errorf("looksLikeGateway(%q) = %v, want %v", command, got, want)
		}
	}
}
