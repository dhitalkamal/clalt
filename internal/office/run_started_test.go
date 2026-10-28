package office

import (
	"context"
	"io"
	"testing"
)

func TestRunCallsStartedWithThePortAndToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan [2]any, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			NoOpen: true, Out: io.Discard, Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
			Started: func(port int, tok string) { got <- [2]any{port, tok} },
		})
	}()
	g := <-got
	if g[0].(int) == 0 || len(g[1].(string)) != 32 {
		t.Errorf("started %v", g)
	}
	cancel()
	if err := <-done; err != nil {
		t.Error(err)
	}
}
