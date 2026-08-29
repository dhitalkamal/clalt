package scheduler

import (
	"strings"
	"testing"
)

func TestPlanPerBackend(t *testing.T) {
	j := testJob()
	cases := []struct {
		name string
		s    Scheduler
		want []string
	}{
		{"launchd", launchdSched{}, []string{"com.clalt.keepalive", "keepalive"}},
		{"systemd", systemdSched{}, []string{".service", ".timer", "systemctl", "OnUnitActiveSec"}},
		{"schtasks", taskschedSched{}, []string{"schtasks", "/Create", "com.clalt.keepalive"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := c.s.Plan(j)
			if strings.TrimSpace(out) == "" {
				t.Fatal("empty plan")
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("plan missing %q:\n%s", w, out)
				}
			}
		})
	}
}
