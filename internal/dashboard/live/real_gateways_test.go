package live

import (
	"context"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/crew"
)

// psRunner answers ps with one line per pid, as ps -o lstart=,command= would
type psRunner struct{ out map[string]string }

func (r psRunner) Output(_ context.Context, s crew.Spec) ([]byte, []byte, error) {
	return []byte(r.out[s.Args[len(s.Args)-1]]), nil, nil
}
func (psRunner) Start(crew.Spec, string) (int, error) { return 0, nil }

func TestGatewaysComeFromTheCrewRecords(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const start = "Thu Oct 23 15:05:12 2026"
	for id, rec := range map[string]crew.Record{
		"work":  {PID: 4242, Port: 8080, ProcStart: start},
		"other": {PID: 4343, Port: 8081, ProcStart: start},
	} {
		if err := crew.WriteRecord(id, rec); err != nil {
			t.Fatal(err)
		}
	}
	m := crew.NewManager(crew.Options{
		Environ: func() []string { return nil },
		Now:     func() time.Time { return epoch },
		Runner: psRunner{out: map[string]string{
			"4242": start + " /usr/bin/kirocrew gateway --port 8080\n",
			// same pid, another process now: stale, not a gateway
			"4343": "Fri Oct 24 01:00:00 2026 /usr/bin/vim\n",
		}},
	})
	got, err := (CrewGateways{List: m.Gateways}).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AccountID != "work" || got[0].Addr != "127.0.0.1:8080" {
		t.Errorf("gateways = %+v", got)
	}
}

func TestNoCrewRecordsAreNoGateways(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := (CrewGateways{}).Running(context.Background())
	if err != nil || len(got) != 0 {
		t.Errorf("gateways = %+v err = %v", got, err)
	}
}
