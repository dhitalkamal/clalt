package usage

import (
	"math"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func rec(ts string, in, out int64, model string) Record {
	return Record{Timestamp: at(ts), Input: in, Output: out, Model: model}
}

func TestWindowFiltersByDuration(t *testing.T) {
	now := at("2026-08-24T12:00:00Z")
	recs := []Record{
		rec("2026-08-24T11:30:00Z", 10, 10, "claude-sonnet-5"), // in 5h window
		rec("2026-08-24T06:00:00Z", 5, 5, "claude-sonnet-5"),   // outside 5h, inside 7d
		rec("2026-08-10T08:00:00Z", 1, 1, "claude-sonnet-5"),   // outside both
	}
	five := Window(recs, now, 5*time.Hour)
	if five.Messages != 1 || five.Tokens != 20 {
		t.Errorf("5h window = %+v, want 1 msg / 20 tokens", five)
	}
	// resets 5h after the earliest record in the window
	wantReset := at("2026-08-24T11:30:00Z").Add(5 * time.Hour)
	if !five.ResetsAt.Equal(wantReset) {
		t.Errorf("ResetsAt = %v, want %v", five.ResetsAt, wantReset)
	}
	seven := Window(recs, now, 7*24*time.Hour)
	if seven.Messages != 2 || seven.Tokens != 30 {
		t.Errorf("7d window = %+v, want 2 msg / 30 tokens", seven)
	}
}

func TestCostOf(t *testing.T) {
	// opus-4-8: 1M input @ $5 => exactly $5 for 1,000,000 input tokens (see
	// cost_test.go for the full priced-model table backing this number)
	r := Record{Input: 1_000_000, Model: "claude-opus-4-8"}
	if c := CostOf(r); math.Abs(c-5.0) > 1e-9 {
		t.Errorf("opus-4-8 1M input cost = %f, want 5", c)
	}
	// unknown model falls back to a nonzero default, never panics
	if c := CostOf(Record{Input: 1_000_000, Model: "mystery"}); c <= 0 {
		t.Errorf("unknown model cost = %f, want > 0", c)
	}
}

func TestTotalCost(t *testing.T) {
	recs := []Record{
		{Input: 1_000_000, Model: "claude-opus-4-8"},  // $5
		{Output: 1_000_000, Model: "claude-opus-4-8"}, // $25
	}
	if c := TotalCost(recs); math.Abs(c-30.0) > 1e-9 {
		t.Errorf("TotalCost = %f, want 30", c)
	}
}

func TestAttribute(t *testing.T) {
	events := []SwitchEvent{
		{Ts: at("2026-08-24T10:00:00Z"), Account: "work"},
		{Ts: at("2026-08-24T11:00:00Z"), Account: "personal"},
	}
	recs := []Record{
		rec("2026-08-24T09:30:00Z", 1, 1, "m"), // before any switch -> unattributed
		rec("2026-08-24T10:30:00Z", 1, 1, "m"), // work era
		rec("2026-08-24T11:30:00Z", 1, 1, "m"), // personal era
		rec("2026-08-24T12:30:00Z", 1, 1, "m"), // still personal
	}
	got := Attribute(recs, events)
	if len(got[Unattributed]) != 1 {
		t.Errorf("unattributed = %d, want 1", len(got[Unattributed]))
	}
	if len(got["work"]) != 1 {
		t.Errorf("work = %d, want 1", len(got["work"]))
	}
	if len(got["personal"]) != 2 {
		t.Errorf("personal = %d, want 2", len(got["personal"]))
	}
}

func TestDailyTrend(t *testing.T) {
	now := at("2026-08-24T12:00:00Z")
	recs := []Record{
		rec("2026-08-24T09:00:00Z", 100, 0, "claude-sonnet-5"), // today
		rec("2026-08-24T11:00:00Z", 100, 0, "claude-sonnet-5"), // today
		rec("2026-08-23T09:00:00Z", 100, 0, "claude-sonnet-5"), // yesterday
	}
	buckets := DailyTrend(recs, 3, now)
	if len(buckets) != 3 {
		t.Fatalf("got %d buckets, want 3", len(buckets))
	}
	// chronological: oldest first, newest (today) last
	last := buckets[len(buckets)-1]
	if last.Tokens != 200 || last.Messages != 2 {
		t.Errorf("today bucket = %+v, want 200 tokens / 2 msgs", last)
	}
	mid := buckets[len(buckets)-2]
	if mid.Tokens != 100 {
		t.Errorf("yesterday tokens = %d, want 100", mid.Tokens)
	}
}
