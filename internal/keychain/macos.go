//go:build darwin

package keychain

import gokeychain "github.com/keybase/go-keychain"

// macStore is the real macOS Keychain implementation. Secrets go in via the
// Security.framework, so they never appear on a command line the way
// `security add-generic-password -w <secret>` would.
type macStore struct{}

// newNative returns the real macOS Keychain store.
func newNative() Store { return macStore{} }

func baseItem(service, account string) gokeychain.Item {
	it := gokeychain.NewItem()
	it.SetSecClass(gokeychain.SecClassGenericPassword)
	it.SetService(service)
	it.SetAccount(account)
	it.SetSynchronizable(gokeychain.SynchronizableNo)
	it.SetAccessible(gokeychain.AccessibleWhenUnlocked)
	return it
}

func (macStore) Get(service, account string) ([]byte, error) {
	q := baseItem(service, account)
	q.SetMatchLimit(gokeychain.MatchLimitOne)
	q.SetReturnData(true)
	results, err := gokeychain.QueryItem(q)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrNotFound
	}
	return results[0].Data, nil
}

// Set creates the item, or updates its data in place if it already exists. The
// update-in-place path is deliberate and load-bearing for the shared
// "Claude Code-credentials" item: SecItemUpdate changes only the data and leaves
// the item's access ACL untouched, so whatever apps Claude Code trusted to read
// it (the claude binary itself) stay trusted. A delete-then-add would reset the
// ACL to trust only clalt and lock claude out, making macOS re-prompt claude on
// every read. Never replace this with Delete+Set for the live credential.
func (macStore) Set(service, account string, data []byte) error {
	it := baseItem(service, account)
	it.SetData(data)
	err := gokeychain.AddItem(it)
	if err == gokeychain.ErrorDuplicateItem {
		q := gokeychain.NewItem()
		q.SetSecClass(gokeychain.SecClassGenericPassword)
		q.SetService(service)
		q.SetAccount(account)
		upd := gokeychain.NewItem()
		upd.SetData(data)
		return gokeychain.UpdateItem(q, upd)
	}
	return err
}

func (macStore) Delete(service, account string) error {
	err := gokeychain.DeleteGenericPasswordItem(service, account)
	if err == gokeychain.ErrorItemNotFound {
		return nil // deleting a missing item is not an error
	}
	return err
}

func (macStore) List(service string) ([]string, error) {
	accounts, err := gokeychain.GetGenericPasswordAccounts(service)
	if err != nil {
		return nil, err
	}
	return accounts, nil
}
