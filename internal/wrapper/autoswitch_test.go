package wrapper

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/pick"
)

func TestAutoSwitchMovesAnIdleSessionThatIsRunningLow(t *testing.T) {
	r := newRig(t)
	r.acc.cands = []pick.Candidate{
		{ID: "work", Label: "work", SignedIn: true, AutoSwitch: true},
		{ID: "personal", Label: "personal", SignedIn: true, AutoSwitch: true},
	}
	r.feed.limits["work"] = lim(93, 40)
	r.feed.limits["personal"] = lim(20, 30)
	r.run()
	r.idle()

	waitFor(t, "the automatic move", func() bool { return len(r.launch.started()) == 2 })

	if got := r.launch.started()[1].folder; got != "/f/personal" {
		t.Errorf("moved to %s", got)
	}
	notice := r.noticeText()
	if !strings.Contains(notice, "from work to personal") || !strings.Contains(notice, "5-hour window at 93%") {
		t.Errorf("the notice: %q", notice)
	}
}

func TestAutoSwitchDoesNothingWhileItIsOffOrNothingHasMoreRoom(t *testing.T) {
	for name, set := range map[string]func(r *rig){
		"off for the account": func(r *rig) {
			r.acc.cands[0].AutoSwitch = false
			r.acc.cands[1].AutoSwitch = true
		},
		"nothing with more room": func(r *rig) {
			r.acc.cands[0].AutoSwitch, r.acc.cands[1].AutoSwitch = true, true
			r.feed.limits["personal"] = lim(95, 10)
		},
	} {
		r := newRig(t)
		r.feed.limits["work"] = lim(93, 40)
		r.feed.limits["personal"] = lim(20, 30)
		set(r)
		r.run()
		r.idle()
		settle()
		if n := len(r.launch.started()); n != 1 {
			t.Errorf("%s: moved", name)
		}
		r.endWithoutMoving()
	}
}

func TestAutoSwitchWaitsOutTheCooldownAfterAMoveBeforeMovingBack(t *testing.T) {
	r := newRig(t)
	r.acc.cands = []pick.Candidate{
		{ID: "work", Label: "work", SignedIn: true, AutoSwitch: true},
		{ID: "personal", Label: "personal", SignedIn: true, AutoSwitch: true},
	}
	r.feed.limits["work"] = lim(93, 40)
	r.feed.limits["personal"] = lim(20, 30)
	r.run()
	r.idle()
	waitFor(t, "the first move", func() bool { return len(r.launch.started()) == 2 })

	// now personal is the one that is low, and work has all its room back
	r.feed.limits["personal"] = lim(95, 40)
	r.feed.limits["work"] = lim(5, 40)
	r.idle()
	settle()
	if n := len(r.launch.started()); n != 2 {
		t.Fatalf("moved back at once: %d starts", n)
	}

	r.clk.Advance(31 * time.Minute)
	r.idle()
	waitFor(t, "the move back after the cooldown", func() bool { return len(r.launch.started()) == 3 })
}

func TestAnAutoSwitchTargetThatFailsIsNotTriedAgainAtOnce(t *testing.T) {
	r := newRig(t)
	r.acc.cands = []pick.Candidate{
		{ID: "work", Label: "work", SignedIn: true, AutoSwitch: true},
		{ID: "personal", Label: "personal", SignedIn: true, AutoSwitch: true},
	}
	r.acc.prepare = func(id, cwd string) (Prepared, error) { return Prepared{}, errors.New("not signed in") }
	r.feed.limits["work"] = lim(93, 40)
	r.feed.limits["personal"] = lim(20, 30)
	r.run()
	r.idle()
	waitFor(t, "the first attempt", func() bool { return len(r.acc.calls()) == 1 })
	settle()

	if n := len(r.acc.calls()); n != 1 {
		t.Errorf("prepared %d times: a failing target is tried at every poll", n)
	}
}

func TestAnAutoMoveIsNotMadeWhenARequestIsWaiting(t *testing.T) {
	r := newRig(t)
	r.acc.cands = []pick.Candidate{
		{ID: "work", Label: "work", SignedIn: true, AutoSwitch: true},
		{ID: "personal", Label: "personal", SignedIn: true, AutoSwitch: true},
		{ID: "third", Label: "third", SignedIn: true, AutoSwitch: true},
	}
	r.feed.limits["work"] = lim(93, 40)
	r.feed.limits["personal"] = lim(20, 30)
	r.feed.limits["third"] = lim(1, 1)
	r.run()
	r.request("personal")
	r.idle()

	waitFor(t, "the move", func() bool { return len(r.launch.started()) == 2 })

	if got := r.launch.started()[1].folder; got != "/f/personal" {
		t.Errorf("moved to %s: the request names personal", got)
	}
	if got := r.noticeText(); !reflect.DeepEqual(strings.Contains(got, "requested with clalt move"), true) {
		t.Errorf("the notice: %q", got)
	}
}

// a figure from long ago is not a sign that the account is low now
func TestAStaleFigureForTheCurrentAccountDoesNotTrigger(t *testing.T) {
	in := input()
	stale := lim(95, 40)
	stale.ObservedAt = clock.Add(-25 * time.Hour)
	in.Limits = func(id string) (pick.Limits, bool) {
		if id == "work" {
			return stale, true
		}
		return lim(10, 10), true
	}
	if d, ok := Decide(in); ok {
		t.Errorf("moved on a figure from 25 hours ago: %+v", d)
	}
}
