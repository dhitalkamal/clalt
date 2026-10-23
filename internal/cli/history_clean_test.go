package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/profile"
)

// the error of a look at projects names the path, which can hold anything a
// file system allows, so what is said about it is cleaned where it is built
func TestWhatIsSaidAboutAnUnlookableHistoryCleansThePath(t *testing.T) {
	newEnv(t)
	parent := filepath.Join(t.TempDir(), "a\x1b[31mb")
	account := filepath.Join(parent, "account")
	if err := os.MkdirAll(account, 0o755); err != nil {
		t.Skipf("this file system refuses the name: %v", err)
	}
	if !makeUnlookable(t, parent) {
		t.Skip("no permission bits to take away")
	}

	problem := projectsProblem(profile.Report{}, account)
	finding, _ := historyFinding("work", account, nil, false)

	for name, got := range map[string]string{"run's refusal": problem, "doctor's finding": finding} {
		if hasControl(got) || !strings.Contains(got, "permission denied") {
			t.Errorf("%s: %q", name, got)
		}
	}
}
