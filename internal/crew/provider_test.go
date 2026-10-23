package crew

import (
	"context"
	"os"
	"slices"
	"testing"
)

func TestGatewaysGivesAccountPortAndStateForTheDashboard(t *testing.T) {
	w := newWorld(t)
	for _, id := range []string{"work", "personal"} {
		if _, err := start(t, w, id); err != nil {
			t.Fatal(err)
		}
	}
	w.kill(4243)
	got, err := w.manager().Gateways(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Gateway{
		{Account: "personal", Port: 5501, State: StateStale},
		{Account: "work", Port: 5500, State: StateRunning},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Gateways = %+v, want %+v", got, want)
	}
}

func TestAGatewayWhoseRecordCannotBeReadIsListedAsBroken(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Record, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := w.manager().Gateways(context.Background())
	if err != nil || !slices.Equal(got, []Gateway{{Account: "work", State: StateBroken}}) {
		t.Errorf("Gateways = %+v, %v", got, err)
	}
}

func TestTheDefaultProviderNeedsNoSetupAndRunsNothingWhenThereIsNoGateway(t *testing.T) {
	isolate(t)
	t.Setenv("CLALT_KIROCREW_BIN", "/nonexistent/kirocrew")
	got, err := Gateways(context.Background())
	if err != nil || len(got) != 0 {
		t.Errorf("Gateways = %+v, %v", got, err)
	}
}
