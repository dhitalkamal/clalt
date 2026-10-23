package crew

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// checkBind looks, once the gateway answers, at what it listens on. a gateway
// is for localhost only, so one that listens anywhere else is stopped and
// reported. when lsof cannot say, the gateway stays and the result carries a
// warning: that is not proof of anything either way.
func (m *Manager) checkBind(ctx context.Context, res StartResult) (StartResult, error) {
	rec := res.Record
	listeners := m.o.Listeners
	if listeners == nil {
		listeners = m.lsof
	}
	addrs, err := listeners(ctx, rec.PID)
	if err != nil || len(addrs) == 0 {
		why := "lsof found no listener"
		if err != nil {
			why = err.Error()
		}
		res.Warning = joinWarning(res.Warning, "could not confirm that the gateway listens on loopback only: "+why)
		return res, nil
	}
	var outside []string
	for _, a := range addrs {
		if !strings.HasPrefix(a, "127.0.0.1:") && !strings.HasPrefix(a, "[::1]:") {
			outside = append(outside, a)
		}
	}
	if len(outside) == 0 {
		return res, nil
	}
	_ = m.locked(func() error {
		_, err := m.stop(ctx, rec.Account)
		return err
	})
	return res, fmt.Errorf("the gateway listens on %s, not on loopback only, so it was stopped", strings.Join(outside, ", "))
}

// lsof lists the addresses a process listens on.
func (m *Manager) lsof(ctx context.Context, pid int) ([]string, error) {
	spec := Spec{Bin: "lsof", Args: []string{"-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:LISTEN", "-Fn"}, Env: m.o.Environ()}
	stdout, _, err := m.o.Runner.Output(ctx, spec)
	if err != nil && len(bytes.TrimSpace(stdout)) == 0 {
		return nil, fmt.Errorf("lsof could not be run: %w", err)
	}
	var addrs []string
	for _, line := range strings.Split(string(stdout), "\n") {
		if a, ok := strings.CutPrefix(strings.TrimSpace(line), "n"); ok && a != "" {
			addrs = append(addrs, a)
		}
	}
	return addrs, nil
}

func joinWarning(a, b string) string {
	if a == "" {
		return b
	}
	return a + ". " + b
}
