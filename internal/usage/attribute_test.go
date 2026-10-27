package usage

import (
	"reflect"
	"testing"
)

// the switch log stopped growing when accounts became folders, so a record
// from after that is not the work of whichever account the last event names.
// from the cutover on, usage is nobody's yet.
func TestAttributeUntilLeavesWhatCameAfterTheCutoverUnattributed(t *testing.T) {
	events := []SwitchEvent{
		{Ts: at("2026-08-24T10:00:00Z"), Account: "work"},
		{Ts: at("2026-08-24T11:00:00Z"), Account: "personal"},
	}
	recs := []Record{
		rec("2026-08-24T09:30:00Z", 1, 1, "m"), // before any switch
		rec("2026-08-24T10:30:00Z", 1, 1, "m"), // work era
		rec("2026-08-24T11:30:00Z", 1, 1, "m"), // personal era, before the cutover
		rec("2026-08-24T12:00:00Z", 1, 1, "m"), // at the cutover
		rec("2026-08-24T12:30:00Z", 1, 1, "m"), // after it, and the last event still says personal
	}

	got := AttributeUntil(recs, events, at("2026-08-24T12:00:00Z"))

	if n := len(got[Unattributed]); n != 3 {
		t.Errorf("unattributed = %d, want 3 (before any switch, at the cutover, after it)", n)
	}
	if n := len(got["work"]); n != 1 {
		t.Errorf("work = %d, want 1", n)
	}
	if n := len(got["personal"]); n != 1 {
		t.Errorf("personal = %d, want 1: only what was before the cutover", n)
	}
}

// without a cutover nothing has been written by the new design yet, and the log
// is all there is
func TestAttributeUntilWithNoCutoverIsAttribute(t *testing.T) {
	events := []SwitchEvent{{Ts: at("2026-08-24T10:00:00Z"), Account: "work"}}
	recs := []Record{
		rec("2026-08-24T09:30:00Z", 1, 1, "m"),
		rec("2026-08-24T10:30:00Z", 1, 1, "m"),
		rec("2027-01-01T00:00:00Z", 1, 1, "m"),
	}

	if got, want := AttributeUntil(recs, events, at("0001-01-01T00:00:00Z")), Attribute(recs, events); !reflect.DeepEqual(got, want) {
		t.Errorf("AttributeUntil = %v, want %v", got, want)
	}
	if n := len(Attribute(recs, events)["work"]); n != 2 {
		t.Errorf("work = %d, want 2: the later record belongs to the last switch, as before", n)
	}
}
