package summary

import (
	"math"
	"sort"
)

// result caps of one stats block.
const (
	maxFiles    = 20
	maxTools    = 30
	maxFailures = 5
	// an idle gap longer than this is not time active
	idleGap = 300
)

// acc collects the numbers of one account or project while events are read.
type acc struct {
	sessions  map[string]bool
	tokens    int64
	cost      float64
	activeSec int64
	toolCalls int
	subs      map[string]int
	tools     map[string]int
	files     map[string]int
	fails     map[[2]string]int
	failN     int
}

func newAcc() *acc {
	return &acc{sessions: map[string]bool{}, subs: map[string]int{}, tools: map[string]int{}, files: map[string]int{}, fails: map[[2]string]int{}}
}

func (a *acc) event(e event) {
	switch e.kind {
	case evSubagent:
		a.subs[e.name]++
	case evFailure:
		a.failN++
		a.fails[[2]string{e.name, e.text}]++
	default:
		a.toolCalls++
		a.tools[e.name]++
		if e.file != "" {
			a.files[e.file]++
		}
	}
}

// counts sorts a tally by count, then name, and keeps the top n.
func counts(m map[string]int, n int) []Count {
	out := make([]Count, 0, len(m))
	for k, v := range m {
		out = append(out, Count{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func (a *acc) stats() Stats {
	fails := make([]Failure, 0, len(a.fails))
	for k, v := range a.fails {
		fails = append(fails, Failure{Tool: k[0], Text: k[1], Count: v})
	}
	sort.Slice(fails, func(i, j int) bool {
		x, y := fails[i], fails[j]
		if x.Count != y.Count {
			return x.Count > y.Count
		}
		if x.Tool != y.Tool {
			return x.Tool < y.Tool
		}
		return x.Text < y.Text
	})
	if len(fails) > maxFailures {
		fails = fails[:maxFailures]
	}
	return Stats{
		Sessions: len(a.sessions), Tokens: a.tokens, Cost: round4(a.cost),
		ActiveMinutes: int(math.Round(float64(a.activeSec) / 60)), ToolCalls: a.toolCalls,
		Subagents: counts(a.subs, maxTools), Tools: counts(a.tools, maxTools), Files: counts(a.files, maxFiles),
		FailureCount: a.failN, Failures: fails,
	}
}

func round4(f float64) float64 { return math.Round(f*1e4) / 1e4 }
