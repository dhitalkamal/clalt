package cli

import (
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office"
)

type limitsByID map[string]office.Limits

func (l limitsByID) Limits(id string) (office.Limits, bool) { v, ok := l[id]; return v, ok }

func TestReviewCandidatesCarryFiveHourPercent(t *testing.T) {
	now := time.Unix(1000, 0)
	accts := []office.Account{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "gone"}}
	lim := limitsByID{
		"a": {FiveHour: &office.Window{UsedPct: 40, ResetsAt: now.Add(time.Hour)}},
		// a window that has reset says nothing about now
		"b": {FiveHour: &office.Window{UsedPct: 99, ResetsAt: now.Add(-time.Minute)}},
	}
	dirs := func(id string) (string, bool) { return "/acc/" + id, id != "gone" }
	got := reviewCandidates(accts, lim, now, dirs)
	if len(got) != 3 {
		t.Fatalf("%+v (an account without a folder is left out)", got)
	}
	if got[0].ID != "a" || got[0].ConfigDir != "/acc/a" || got[0].FiveHourUsedPct == nil || *got[0].FiveHourUsedPct != 40 {
		t.Errorf("a = %+v", got[0])
	}
	if got[1].FiveHourUsedPct != nil || got[2].FiveHourUsedPct != nil {
		t.Errorf("unknown windows must stay nil: %+v %+v", got[1], got[2])
	}
}
