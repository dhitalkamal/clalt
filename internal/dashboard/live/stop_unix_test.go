//go:build !windows

package live

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDefaultProbeReadsThisProcessFromPs(t *testing.T) {
	p, err := probeProc(context.Background(), os.Getpid())
	if err != nil {
		t.Skipf("ps is not usable here: %v", err)
	}
	if !p.Alive || p.PPID != os.Getppid() || p.Command == "" {
		t.Errorf("proc = %+v", p)
	}
	if d := time.Since(p.Start); d < 0 || d > 24*time.Hour {
		t.Errorf("start = %v", p.Start)
	}
}

func TestDefaultProbeSeesAGonePidAsNotAlive(t *testing.T) {
	p, err := probeProc(context.Background(), deadPID(t))
	if err != nil || p.Alive {
		t.Errorf("proc = %+v err = %v", p, err)
	}
}
