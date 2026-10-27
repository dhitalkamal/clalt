package usage

import "strings"

// Pricing is USD per 1,000,000 tokens for each token class. LongPrompt, when
// set, is the rate card that replaces these four rates for a request whose
// prompt is over longPromptTokens (see CostOf); nil means one flat rate card.
// It points into priceTable, so treat it as read-only.
type Pricing struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
	LongPrompt *Pricing
}

// synthetic is Claude Code's marker for a message with no real API call
// behind it (e.g. a locally synthesized turn); it always costs zero.
const synthetic = "<synthetic>"

// longPromptTokens is the prompt size (input + cache read + cache creation
// tokens) above which a model with a long-prompt tier bills at that tier. The
// threshold is exclusive: a prompt of exactly this size is still the base tier.
const longPromptTokens = 100_000

// priceRule pairs a substring pattern with the price to use when a model id
// contains it. priceTable is ordered MOST SPECIFIC FIRST and matched in order,
// first match wins - this matters because some patterns are substrings of
// others (e.g. "opus-5" is a substring of "opus-5-5"), and Go map iteration
// order is random, which was the original bug here.
//
// Prices drift as Anthropic updates its list; when they do, update the
// pricing entries below (and the mirrored table in cost_test.go). Cache write
// is 1.25x input (5-minute cache) and cache read is 0.1x input unless a rule
// below sets a different CacheRead explicitly.
type priceRule struct {
	pattern string
	price   Pricing
}

// price builds a Pricing from list input/output, deriving cache write at
// 1.25x input and cache read at 0.1x input, unless cacheRead is given
// explicitly (used for the handful of models with a cheaper posted cache-read
// rate).
func price(input, output float64, cacheRead ...float64) Pricing {
	cr := input * 0.1
	if len(cacheRead) > 0 {
		cr = cacheRead[0]
	}
	return Pricing{Input: input, Output: output, CacheWrite: input * 1.25, CacheRead: cr}
}

// tiered attaches a long-prompt rate card to a base price.
func tiered(base, long Pricing) Pricing {
	base.LongPrompt = &long
	return base
}

var priceTable = []priceRule{
	// fable / mythos family (mythos-5-1 is priced like fable-5-1)
	{"fable-5-1", price(10, 50, 0.25)}, // explicit cheaper cache read
	{"mythos-5-1", price(10, 50, 0.25)},
	{"fable-5", price(10, 50)},

	// opus family, most specific model ids first
	{"opus-5-5", price(4, 20, 0.20)}, // explicit cheaper cache read
	{"opus-4-8", price(5, 25)},
	{"opus-4-7", price(5, 25)},
	{"opus-4-6", price(5, 25)},
	{"opus-4-5", price(5, 25)},
	{"opus-5", price(5, 25)},
	{"opus-4-1", price(15, 75)},
	{"opus-4", price(15, 75)}, // opus-4-0, date-suffixed opus-4-* not matched above
	{"3-opus", price(15, 75)},

	// sonnet family
	{"sonnet-5", price(2, 10)},

	// haiku family, most specific model ids first
	{"haiku-5", tiered(price(0.10, 0.50), price(0.50, 2.50))}, // steps up above a 100K prompt
	{"haiku-4-5", price(1, 5)},
	{"3-5-haiku", price(0.80, 4)},
	{"3-haiku", price(0.25, 1.25)},

	// generic family fallbacks - order matters: these must come after every
	// specific pattern above for the same family, and are also what unknown
	// ids of a recognized family fall back to.
	{"sonnet", price(3, 15)},
	{"haiku", price(1, 5)},
	{"fable", price(10, 50)},
	{"mythos", price(10, 50)},
	{"opus", price(5, 25)},
}

// defaultPrice is used when a model id matches nothing known at all (not even
// a family keyword): falls back to Sonnet-tier so cost is never zero for real
// usage.
var defaultPrice = price(3, 15)

// PriceFor returns the base pricing tier for a model id (a model with a
// long-prompt tier carries it in LongPrompt; CostOf decides when it applies).
// Matching is ordered, most-specific-pattern-first (see priceTable) so it is
// deterministic regardless of how many patterns a given id happens to contain.
func PriceFor(model string) Pricing {
	if model == synthetic {
		return Pricing{}
	}
	m := strings.ToLower(model)
	for _, rule := range priceTable {
		if strings.Contains(m, rule.pattern) {
			return rule.price
		}
	}
	return defaultPrice
}

// CostOf estimates the USD cost of a single record. The whole record bills at
// the model's long-prompt tier when it has one and the record's prompt (input +
// cache read + cache creation tokens) is over longPromptTokens.
func CostOf(r Record) float64 {
	p := PriceFor(r.Model)
	if p.LongPrompt != nil && r.Input+r.CacheRead+r.CacheCreate > longPromptTokens {
		p = *p.LongPrompt
	}
	const perM = 1_000_000.0
	return float64(r.Input)/perM*p.Input +
		float64(r.Output)/perM*p.Output +
		float64(r.CacheCreate)/perM*p.CacheWrite +
		float64(r.CacheRead)/perM*p.CacheRead
}

// TotalCost sums CostOf across records.
func TotalCost(records []Record) float64 {
	var sum float64
	for _, r := range records {
		sum += CostOf(r)
	}
	return sum
}
