package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// LiveWindow is one authoritative usage window from the live endpoint.
type LiveWindow struct {
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resets_at"`
}

// LiveUsage is the authoritative usage payload.
type LiveUsage struct {
	FiveHour LiveWindow `json:"five_hour"`
	SevenDay LiveWindow `json:"seven_day"`
}

// CacheEntry is a cached live response with its fetch time.
type CacheEntry struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Data      LiveUsage `json:"data"`
}

// Fresh reports whether the entry is still within ttl as of now. Stale entries
// remain usable as a fallback when the live endpoint 429s.
func (e CacheEntry) Fresh(ttl time.Duration, now time.Time) bool {
	return now.Sub(e.FetchedAt) < ttl
}

// Cache stores live usage responses on disk to respect the endpoint's aggressive
// rate limiting.
type Cache struct{ dir string }

// NewCache builds a Cache rooted at dir.
func NewCache(dir string) *Cache { return &Cache{dir: dir} }

func (c *Cache) path(name string) string {
	return filepath.Join(c.dir, "usage-"+name+".json")
}

// Save writes the entry for an account.
func (c *Cache) Save(name string, data LiveUsage, at time.Time) error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(CacheEntry{FetchedAt: at, Data: data}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(name), raw, 0o600)
}

// Load returns the cached entry for an account and whether one exists.
func (c *Cache) Load(name string) (CacheEntry, bool) {
	raw, err := os.ReadFile(c.path(name))
	if err != nil {
		return CacheEntry{}, false
	}
	var e CacheEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		return CacheEntry{}, false
	}
	return e, true
}
