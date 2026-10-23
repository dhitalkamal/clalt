package live

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

func TestRemovedAccountsLoadAsRemovedRowsAndStayOutOfTheAccounts(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	gone := r.signedIn("gone", "g@example.com")
	r.store.Hide(gone)
	// hiding clears the identity, so a record that still has an email is set after
	r.store.SetIdentity(gone, account.Identity{Email: "g@example.com"})

	snap := r.load(r.adapter())

	if len(snap.Accounts) != 1 || snap.Accounts[0].Label != "work" {
		t.Fatalf("a removed account is not an account: %+v", snap.Accounts)
	}
	if len(snap.Removed) != 1 {
		t.Fatalf("removed = %+v", snap.Removed)
	}
	got := snap.Removed[0]
	if got.ID != gone || got.Label != "gone" || got.Email != "g@example.com" || got.Folder == "" {
		t.Errorf("removed = %+v", got)
	}
}

func TestNoRemovedAccountsIsNoRemovedRows(t *testing.T) {
	r := newRig(t)
	r.signedIn("work", "w@example.com")
	if snap := r.load(r.adapter()); len(snap.Removed) != 0 {
		t.Errorf("removed = %+v", snap.Removed)
	}
}

func TestRemovedAccountsAreNotChecked(t *testing.T) {
	r := newRig(t)
	gone := r.signedIn("gone", "g@example.com")
	r.store.Hide(gone)
	r.load(r.adapter())
	if n := r.host.checks.Load(); n != 0 {
		t.Errorf("a removed account is never asked about: %d checks", n)
	}
}

func TestDeleteAndSignInAcceptARemovedAccount(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("gone", "g@example.com")
	r.store.Hide(id)
	a := r.adapter()

	if err := a.Delete(bg, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(r.host.deleted) != 1 || r.host.deleted[0] != id {
		t.Errorf("host deletes = %v", r.host.deleted)
	}
	if _, err := a.SignIn(bg, id); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if len(r.host.launches) != 1 {
		t.Errorf("launches = %v", r.host.launches)
	}
}

func TestRunStillRefusesARemovedAccount(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("gone", "g@example.com")
	r.store.Hide(id)
	if _, err := r.adapter().Run(bg, id); err == nil {
		t.Error("a removed account is not run from the dashboard")
	}
}
