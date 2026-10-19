// Package usage computes local (and optionally live) Claude usage stats: it
// parses Claude Code's transcript JSONL, rolls it into 5h/7d windows, estimates
// cost, attributes activity to accounts, and builds daily trends.
package usage

import "time"

// Record is one assistant message's token usage from a transcript.
type Record struct {
	Timestamp   time.Time
	Model       string
	Input       int64
	Output      int64
	CacheCreate int64
	CacheRead   int64
	Project     string
	SessionID   string
	Cwd         string // working dir the record was written from; used for --continue/-c session discovery
	MessageID   string // message.id: repeats across content-block lines of the same reply, used to dedup
	RequestID   string // top-level requestId: paired with MessageID for the dedup key
}

// Billable is input + output tokens: the headline "tokens" number. Cache reads
// are deliberately excluded here because with a high cache-hit rate they dwarf
// real input/output by ~30x and make totals look absurd. Cache is surfaced
// separately as a composition breakdown and hit rate.
func (r Record) Billable() int64 {
	return r.Input + r.Output
}

// TotalTokens is the sum across all token classes (used for composition and the
// cache-hit ratio, not for the headline token count).
func (r Record) TotalTokens() int64 {
	return r.Input + r.Output + r.CacheCreate + r.CacheRead
}
