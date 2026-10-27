package cli

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// a moved session resumes with the same claude session id. usage after the move
// belongs to the new account, so the move has to leave a line of its own
func TestHookAttributesAResumedSessionToTheNewAccount(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	reg := registry.Registry{Dir: config.SessionsDir()}
	e.hook("SessionStart", `{"session_id":"one"}`)
	e.hook("Stop", `{"session_id":"one"}`)

	if _, _, err := reg.Update(h, func(r *registry.Record) { r.Account = "other" }); err != nil {
		t.Fatal(err)
	}
	e.hook("SessionStart", `{"session_id":"one","source":"resume"}`)
	e.hook("Stop", `{"session_id":"one"}`)

	got, err := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if err != nil || len(got) != 2 {
		t.Fatalf("lines = %+v, %v, want 2", got, err)
	}
	if got[0].Account != "work" || got[1].Account != "other" || got[1].SessionID != "one" {
		t.Errorf("lines = %+v", got)
	}
}
