package cli

import (
	"os"
	"runtime"

	"github.com/dhitalkamal/clalt/internal/runcmd"
	"github.com/dhitalkamal/clalt/internal/terminal"
)

// sessionDeps is what resume, launch, jump and notify reach outside clalt with:
// the programs they run, the environment of the shell, and what step 3a's
// registry and feed know. App.sdeps nil means the real ones; tests set fakes.
type sessionDeps struct {
	runner runcmd.Runner
	env    func(name string) string
	exe    func() (string, error)
	goos   string

	// the running sessions, dead ones left out, from the session registry.
	sessions func() ([]terminal.Session, error)
	// the account id that last ran a claude session, when the attribution log
	// knows.
	lastAccount func(claudeSessionID string) (accountID string, ok bool)
}

// newRunner makes the runner of the real deps. a variable so that the tests of
// this package can keep every outside program out of reach, see
// session_deps_test.go.
var newRunner = func() runcmd.Runner { return runcmd.Exec{} }

func realDeps() *sessionDeps {
	return &sessionDeps{
		runner:      newRunner(),
		env:         os.Getenv,
		exe:         os.Executable,
		goos:        runtime.GOOS,
		sessions:    registrySessions,
		lastAccount: attributedAccount,
	}
}

func (a *App) deps() *sessionDeps {
	if a.sdeps == nil {
		a.sdeps = realDeps()
	}
	return a.sdeps
}
