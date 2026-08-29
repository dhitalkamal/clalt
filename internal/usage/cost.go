package usage

import "strings"

// Pricing is USD per 1,000,000 tokens for each token class.
type Pricing struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
}

// prices are approximate list prices and WILL drift; treat dashboard cost as an
// estimate. Update this single table when Anthropic changes pricing. Matched by
// substring of the model id.
var prices = map[string]Pricing{
	"opus":   {Input: 15, Output: 75, CacheWrite: 18.75, CacheRead: 1.5},
	"sonnet": {Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.30},
	"haiku":  {Input: 0.80, Output: 4, CacheWrite: 1, CacheRead: 0.08},
}

// defaultPrice is used when a model id matches nothing known (falls back to
// Sonnet-tier so cost is never zero for real usage).
var defaultPrice = prices["sonnet"]

// PriceFor returns the pricing tier for a model id.
func PriceFor(model string) Pricing {
	m := strings.ToLower(model)
	for key, p := range prices {
		if strings.Contains(m, key) {
			return p
		}
	}
	return defaultPrice
}

// CostOf estimates the USD cost of a single record.
func CostOf(r Record) float64 {
	p := PriceFor(r.Model)
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
