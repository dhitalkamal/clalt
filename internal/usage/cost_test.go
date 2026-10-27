package usage

import (
	"math"
	"testing"
)

// closeEnough tolerates float64 rounding noise (e.g. 0.1*3 != 0.3 exactly).
func closeEnough(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestPriceForKnownModels is table-driven over every model id and price called
// out in the pricing spec (USD per 1M tokens, input/output). Cache write is
// always 1.25x input; cache read is 0.1x input unless a different value is
// noted in the comment (opus-5-5, fable-5-1 and mythos-5-1 have an explicit
// cheaper cache read rate). Rows give the base tier; haiku-5 also has a
// long-prompt tier, covered by TestCostOfHaiku5LongPromptTier.
func TestPriceForKnownModels(t *testing.T) {
	cases := []struct {
		model          string
		input          float64
		output         float64
		cacheReadWant  float64
		cacheWriteWant float64
	}{
		{"claude-fable-5-1", 10, 50, 0.25, 12.5}, // explicit cheaper cache read
		{"claude-fable-5", 10, 50, 1.0, 12.5},
		{"claude-mythos-5-1", 10, 50, 0.25, 12.5},
		{"claude-opus-5-5", 4, 20, 0.20, 5.0}, // explicit cheaper cache read
		{"claude-opus-5", 5, 25, 0.5, 6.25},
		{"claude-opus-4-8", 5, 25, 0.5, 6.25},
		{"claude-opus-4-7", 5, 25, 0.5, 6.25},
		{"claude-opus-4-6", 5, 25, 0.5, 6.25},
		{"claude-opus-4-5", 5, 25, 0.5, 6.25},
		{"claude-opus-4-1", 15, 75, 1.5, 18.75},
		{"claude-opus-4", 15, 75, 1.5, 18.75},
		{"claude-opus-4-0", 15, 75, 1.5, 18.75},
		{"claude-opus-4-20250514", 15, 75, 1.5, 18.75}, // date-suffixed id
		{"claude-3-opus", 15, 75, 1.5, 18.75},
		{"claude-sonnet-5", 2, 10, 0.2, 2.5},
		{"claude-sonnet-4-6", 3, 15, 0.3, 3.75},
		{"claude-sonnet-4-5", 3, 15, 0.3, 3.75},
		{"claude-sonnet-4", 3, 15, 0.3, 3.75},
		{"claude-sonnet-3-7", 3, 15, 0.3, 3.75},
		{"claude-sonnet-3-5", 3, 15, 0.3, 3.75},
		{"claude-haiku-5-5", 0.10, 0.50, 0.01, 0.125}, // base tier, up to a 100K prompt
		{"claude-haiku-4-5", 1, 5, 0.1, 1.25},
		{"claude-haiku-4-5-20251001", 1, 5, 0.1, 1.25}, // date-suffixed id
		{"claude-3-5-haiku", 0.80, 4, 0.08, 1.0},
		{"claude-3-haiku", 0.25, 1.25, 0.025, 0.3125},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			p := PriceFor(tc.model)
			if !closeEnough(p.Input, tc.input) || !closeEnough(p.Output, tc.output) {
				t.Errorf("PriceFor(%q) input/output = %v/%v, want %v/%v", tc.model, p.Input, p.Output, tc.input, tc.output)
			}
			if !closeEnough(p.CacheRead, tc.cacheReadWant) {
				t.Errorf("PriceFor(%q) cacheRead = %v, want %v", tc.model, p.CacheRead, tc.cacheReadWant)
			}
			if !closeEnough(p.CacheWrite, tc.cacheWriteWant) {
				t.Errorf("PriceFor(%q) cacheWrite = %v, want %v", tc.model, p.CacheWrite, tc.cacheWriteWant)
			}
		})
	}
}

// TestPriceForSynthetic proves the synthetic marker (no real API call) always
// costs zero.
func TestPriceForSynthetic(t *testing.T) {
	p := PriceFor("<synthetic>")
	if p.Input != 0 || p.Output != 0 || p.CacheWrite != 0 || p.CacheRead != 0 {
		t.Errorf("PriceFor(<synthetic>) = %+v, want all zero", p)
	}
	if c := CostOf(Record{Model: "<synthetic>", Input: 1_000_000, Output: 1_000_000}); c != 0 {
		t.Errorf("CostOf(<synthetic>) = %v, want 0", c)
	}
}

// TestPriceForUnknownFallsBackByFamily proves an unrecognized id falls back to
// its family's current price, and a totally unrecognized id falls back to
// sonnet pricing.
func TestPriceForUnknownFallsBackByFamily(t *testing.T) {
	cases := []struct {
		model  string
		input  float64
		output float64
	}{
		{"claude-opus-99", 5, 25},
		{"claude-sonnet-99", 3, 15},
		{"claude-haiku-99", 1, 5},
		{"claude-fable-99", 10, 50},
		{"claude-mythos-99", 10, 50},
		{"totally-unknown-model", 3, 15},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			p := PriceFor(tc.model)
			if p.Input != tc.input || p.Output != tc.output {
				t.Errorf("PriceFor(%q) = %v/%v, want %v/%v", tc.model, p.Input, p.Output, tc.input, tc.output)
			}
		})
	}
}

// TestPriceForOrderingMostSpecificFirst proves opus-5-5 is not shadowed by the
// more general opus-5 pattern (opus-5 is a substring of opus-5-5).
func TestPriceForOrderingMostSpecificFirst(t *testing.T) {
	p55 := PriceFor("claude-opus-5-5")
	p5 := PriceFor("claude-opus-5")
	if p55.Input == p5.Input && p55.Output == p5.Output {
		t.Fatal("opus-5-5 and opus-5 must not resolve to the same price (ordering bug)")
	}
	if p55.Input != 4 || p55.Output != 20 {
		t.Errorf("opus-5-5 = %v/%v, want 4/20", p55.Input, p55.Output)
	}
}

// TestMythos51PricesLikeFable51 proves mythos-5-1 carries fable-5-1's posted
// price, including the cheaper cache read rate.
func TestMythos51PricesLikeFable51(t *testing.T) {
	m, f := PriceFor("claude-mythos-5-1"), PriceFor("claude-fable-5-1")
	if m.Input != f.Input || m.Output != f.Output || m.CacheWrite != f.CacheWrite || m.CacheRead != f.CacheRead {
		t.Errorf("mythos-5-1 = %+v, want the same rates as fable-5-1 = %+v", m, f)
	}
	if got := CostOf(Record{Model: "claude-mythos-5-1", CacheRead: 1_000_000}); !closeEnough(got, 0.25) {
		t.Errorf("CostOf(mythos-5-1, 1M cache read) = %v, want 0.25", got)
	}
}

// TestCostOfHaiku5LongPromptTier covers haiku 5's two tiers: 0.10/0.50 per 1M
// (input/output) up to a 100,000-token prompt, 0.50/2.50 above it. Prompt size
// is input + cache read + cache creation tokens (never output) and the
// threshold is exclusive. Cache rates inside a tier derive from that tier's
// input price (write 1.25x, read 0.1x).
func TestCostOfHaiku5LongPromptTier(t *testing.T) {
	cases := []struct {
		name string
		rec  Record
		want float64
	}{
		{"prompt at threshold stays base", Record{Input: 100_000, Output: 1_000_000}, 0.51},
		{"prompt one over goes long", Record{Input: 100_001, Output: 1_000_000}, 2.5500005},
		{"cache read counts toward prompt", Record{Input: 1, CacheRead: 100_000}, 0.0050005},
		{"cache creation counts toward prompt", Record{Input: 1, CacheCreate: 100_000}, 0.0625005},
		{"output does not count toward prompt", Record{Input: 100_000, Output: 500_000}, 0.26},
		{"mixed classes at threshold stay base", Record{Input: 40_000, CacheRead: 30_000, CacheCreate: 30_000}, 0.00805},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.rec.Model = "claude-haiku-5-5"
			if got := CostOf(tc.rec); !closeEnough(got, tc.want) {
				t.Errorf("CostOf(%+v) = %v, want %v", tc.rec, got, tc.want)
			}
		})
	}
}

// TestHaiku45IsNotCapturedByHaiku5 proves the haiku-5 rule does not shadow
// haiku-4-5 (plain or date-suffixed) and that haiku-4-5 has no long-prompt
// tier: a 200K-token prompt still bills at the flat 1/5 rate.
func TestHaiku45IsNotCapturedByHaiku5(t *testing.T) {
	for _, model := range []string{"claude-haiku-4-5", "claude-haiku-4-5-20251001"} {
		t.Run(model, func(t *testing.T) {
			p := PriceFor(model)
			if p.Input != 1 || p.Output != 5 {
				t.Errorf("PriceFor(%q) = %v/%v, want 1/5", model, p.Input, p.Output)
			}
			if got := CostOf(Record{Model: model, Input: 200_000, Output: 1_000_000}); !closeEnough(got, 5.2) {
				t.Errorf("CostOf(%q, 200K prompt) = %v, want 5.2 (flat rate)", model, got)
			}
		})
	}
}
