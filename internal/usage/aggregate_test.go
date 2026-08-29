package usage

import (
	"math"
	"testing"
)

func TestAggregate(t *testing.T) {
	now := at("2026-08-24T12:00:00Z")
	recs := []Record{
		// today, opus, with cache
		{Timestamp: at("2026-08-24T01:00:00Z"), Model: "claude-opus-4-8", Input: 100, Output: 200, CacheCreate: 50, CacheRead: 9000, SessionID: "s1", Project: "proj-a"},
		{Timestamp: at("2026-08-24T01:30:00Z"), Model: "claude-sonnet-5", Input: 10, Output: 40, CacheRead: 1000, SessionID: "s1", Project: "proj-a"},
		// yesterday, sonnet
		{Timestamp: at("2026-08-23T09:00:00Z"), Model: "claude-sonnet-5", Input: 5, Output: 15, SessionID: "s2", Project: "proj-b"},
	}
	s := Aggregate(recs, now)

	// billable = input + output only, cache excluded from headline
	if s.Billable != 100+200+10+40+5+15 {
		t.Errorf("Billable = %d, want 370", s.Billable)
	}
	// cache tracked separately
	if s.CacheRead != 10000 || s.CacheWrite != 50 {
		t.Errorf("cache = read %d / write %d", s.CacheRead, s.CacheWrite)
	}
	if s.Messages != 3 {
		t.Errorf("Messages = %d, want 3", s.Messages)
	}
	// distinct sessions
	if s.Sessions != 2 {
		t.Errorf("Sessions = %d, want 2", s.Sessions)
	}
	// today = 2 msgs (the two on aug 24)
	if s.Today.Messages != 2 || s.Today.Billable != 350 {
		t.Errorf("Today = %+v, want 2 msgs / 350 billable", s.Today)
	}
	// cache hit rate = cacheRead / grand total (input+output+cacheWrite+cacheRead)
	// = 10000 / (115 + 255 + 50 + 10000) = 10000/10420 ~= 0.9597
	if math.Abs(s.CacheHitRate-0.9597) > 0.01 {
		t.Errorf("CacheHitRate = %f, want ~0.9597", s.CacheHitRate)
	}
	// models sorted by billable desc: opus(300) > sonnet(70)
	if len(s.Models) != 2 || s.Models[0].Model != "claude-opus-4-8" {
		t.Errorf("models = %+v", s.Models)
	}
	// projects sorted by billable desc: proj-a(350) > proj-b(20)
	if len(s.Projects) == 0 || s.Projects[0].Project != "proj-a" {
		t.Errorf("projects = %+v", s.Projects)
	}
}

func TestHourHistogram(t *testing.T) {
	now := at("2026-08-24T12:00:00Z")
	recs := []Record{
		{Timestamp: at("2026-08-24T01:15:00Z"), Input: 1, Output: 1},
		{Timestamp: at("2026-08-24T01:45:00Z"), Input: 1, Output: 1},
		{Timestamp: at("2026-08-23T13:00:00Z"), Input: 1, Output: 1},
	}
	s := Aggregate(recs, now)
	if len(s.HourOfDay) != 24 {
		t.Fatalf("HourOfDay len = %d, want 24", len(s.HourOfDay))
	}
	if s.HourOfDay[1] != 2 {
		t.Errorf("hour 1 count = %d, want 2", s.HourOfDay[1])
	}
	if s.HourOfDay[13] != 1 {
		t.Errorf("hour 13 count = %d, want 1", s.HourOfDay[13])
	}
}

func TestCompositionPercents(t *testing.T) {
	now := at("2026-08-24T12:00:00Z")
	recs := []Record{
		{Timestamp: now, Input: 30, Output: 30, CacheCreate: 40, CacheRead: 900},
	}
	s := Aggregate(recs, now)
	// grand total = 1000; cacheRead 90%, cacheWrite 4%, input 3%, output 3%
	if p := s.Composition(); math.Abs(p.CacheRead-0.9) > 0.001 {
		t.Errorf("cacheRead share = %f, want 0.9", p.CacheRead)
	}
}
