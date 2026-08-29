package usage

import (
	"testing"
	"time"
)

func TestCacheSaveLoad(t *testing.T) {
	c := NewCache(t.TempDir())
	now := at("2026-08-24T12:00:00Z")
	data := LiveUsage{
		FiveHour: LiveWindow{Utilization: 0.42, ResetsAt: now.Add(2 * time.Hour)},
		SevenDay: LiveWindow{Utilization: 0.10},
	}
	if err := c.Save("work", data, now); err != nil {
		t.Fatalf("Save: %v", err)
	}
	e, ok := c.Load("work")
	if !ok {
		t.Fatal("Load returned not-present after Save")
	}
	if e.Data.FiveHour.Utilization != 0.42 {
		t.Errorf("utilization = %f", e.Data.FiveHour.Utilization)
	}
	if !e.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt = %v, want %v", e.FetchedAt, now)
	}
}

func TestCacheFreshness(t *testing.T) {
	c := NewCache(t.TempDir())
	now := at("2026-08-24T12:00:00Z")
	_ = c.Save("work", LiveUsage{}, now)
	e, _ := c.Load("work")
	// within TTL -> fresh
	if !e.Fresh(10*time.Minute, now.Add(5*time.Minute)) {
		t.Error("entry should be fresh 5m in with a 10m TTL")
	}
	// past TTL -> stale (still loadable, just not fresh, for 429 fallback)
	if e.Fresh(10*time.Minute, now.Add(20*time.Minute)) {
		t.Error("entry should be stale 20m in with a 10m TTL")
	}
}

func TestCacheLoadMissing(t *testing.T) {
	c := NewCache(t.TempDir())
	if _, ok := c.Load("ghost"); ok {
		t.Error("Load should report not-present for unknown account")
	}
}
