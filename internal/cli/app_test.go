package cli

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// v0.1.0 created a vault key on every command, even `clalt version`.
func TestCommandsThatNeedNoStateCreateNone(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"help"}, {"list"}, {"whoami"}, {"sessions"}} {
		e := newEnv(t)

		if code := e.run(args...); code != 0 {
			t.Fatalf("%v exited %d: %s", args, code, e.errOut.String())
		}

		if exists(config.ClaltDir()) {
			t.Errorf("%v created %s", args, config.ClaltDir())
		}
	}
}

func TestNewAppOpensNoVault(t *testing.T) {
	e := newEnv(t)

	if exists(config.VaultDir()) || exists(config.VaultKeyPath()) {
		t.Error("NewApp created a vault")
	}
	if e.app.Store == nil {
		t.Error("no store")
	}
}
