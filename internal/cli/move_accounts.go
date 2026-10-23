package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/pick"
	"github.com/dhitalkamal/clalt/internal/rules"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

// moveAccounts is what the wrapper knows of accounts: the ones that were not
// removed with their rules, and how to get one ready to take a session.
type moveAccounts struct{ a *App }

var _ wrapper.Accounts = moveAccounts{}

// Candidates are the visible accounts. a sign-in is "recorded" the way pick
// reads it, and Prepare asks claude.
func (m moveAccounts) Candidates() ([]pick.Candidate, error) {
	accts, err := m.a.visibleAccounts()
	if err != nil {
		return nil, err
	}
	all, err := m.a.Store.AllRules()
	if err != nil {
		return nil, err
	}
	out := make([]pick.Candidate, len(accts))
	for i, acc := range accts {
		r := all[acc.Name]
		out[i] = pick.Candidate{
			ID: acc.Name, Label: acc.Label(), SignedIn: !acc.Meta.SignedInAt.IsZero() && acc.Meta.Email != "",
			Folders: r.Folders, AutoSwitch: r.AutoSwitch,
		}
	}
	return out, nil
}

// Prepare does for a move what `clalt run` does before it starts claude: the
// rules of the account, its folder linked to the shared home, the variables that
// would override its login set aside, and claude's word that it is signed in as
// the person it was set up for. what it says is returned and not printed, since
// claude's screen owns the terminal until it is ended.
func (m moveAccounts) Prepare(id, cwd string) (wrapper.Prepared, error) {
	all, err := m.a.Store.List()
	if err != nil {
		return wrapper.Prepared{}, err
	}
	var acc account.Account
	found := false
	for _, c := range all {
		if c.Name == id {
			acc, found = c, true
		}
	}
	switch {
	case !found:
		return wrapper.Prepared{}, fmt.Errorf("there is no account %s", clean(id))
	case acc.Meta.Hidden:
		return wrapper.Prepared{}, fmt.Errorf("%s was removed", shown(acc))
	}
	r, err := m.a.Store.RulesOf(id)
	if err != nil {
		return wrapper.Prepared{}, err
	}
	if !rules.Allows(r.Folders, cwd) {
		return wrapper.Prepared{}, fmt.Errorf("the folder rules of %s do not include %s", shown(acc), clean(cwd))
	}
	folder, err := folderFor(acc)
	if err != nil {
		return wrapper.Prepared{}, err
	}
	if err := m.a.refuseSharedFolder(acc); err != nil {
		return wrapper.Prepared{}, err
	}

	var said bytes.Buffer
	quiet := *m.a
	quiet.Err = &said
	rep, err := quiet.syncAccount(folder)
	if err != nil {
		return wrapper.Prepared{}, err
	}
	if problem := projectsProblem(rep, folder); problem != "" {
		return wrapper.Prepared{}, errors.New("history would not be shared: " + problem)
	}
	quiet.noteStripped(folder)
	if why := m.a.notReady(acc); why != "" {
		return wrapper.Prepared{}, fmt.Errorf("%s is not ready to take a session: %s", shown(acc), why)
	}
	if err := m.a.Store.Touch(id); err != nil {
		fmt.Fprintf(&said, "warning: could not record the use of %s: %v\n", shown(acc), err)
	}
	prep := wrapper.Prepared{Label: shown(acc), Folder: folder}
	for _, line := range strings.Split(strings.TrimSpace(said.String()), "\n") {
		if line != "" {
			prep.Notes = append(prep.Notes, line)
		}
	}
	return prep, nil
}
