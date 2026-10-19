package dashboard

const (
	weekDays  = 7
	monthDays = 30
)

// usageTotal is a headline sum. tokens are input plus output.
type usageTotal struct {
	tokens int64
	cost   float64
}

func firstDays(days []Day, n int) []Day { return days[:min(n, len(days))] }

func sumDays(days []Day) usageTotal {
	var t usageTotal
	for _, d := range days {
		t.tokens += d.Input + d.Output
		t.cost += d.Cost
	}
	return t
}

// costsOldestFirst is the order a sparkline reads, days arrive newest first
func costsOldestFirst(days []Day) []float64 {
	out := make([]float64, len(days))
	for i, d := range days {
		out[len(days)-1-i] = d.Cost
	}
	return out
}

func (a Account) today() usageTotal {
	return sumDays(firstDays(a.History.Days, 1))
}

func (a Account) week() usageTotal {
	return sumDays(firstDays(a.History.Days, weekDays))
}

func (a Account) monthCost() float64 {
	return sumDays(firstDays(a.History.Days, monthDays)).cost
}

func (a Account) weekCosts() []float64 {
	return costsOldestFirst(firstDays(a.History.Days, weekDays))
}

func (a Account) monthCosts() []float64 {
	return costsOldestFirst(firstDays(a.History.Days, monthDays))
}

// loginLevel warns three days before the 30 day cap and goes red at one
func loginLevel(dueDays int) Level {
	switch {
	case dueDays <= 1:
		return LevelBad
	case dueDays <= 3:
		return LevelWarn
	}
	return LevelOK
}
