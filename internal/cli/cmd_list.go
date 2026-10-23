package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
)

// statusWorkers is how many `claude auth status` calls run at once.
const statusWorkers = 4

// accountRow is one account as `list` and `whoami --json` show it. signedIn is
// null when claude could not be asked, and reason says why.
type accountRow struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Email      string     `json:"email"`
	Org        string     `json:"org"`
	Plan       string     `json:"plan"`
	SignedIn   *bool      `json:"signedIn"`
	Reason     string     `json:"reason"`
	SignedInAt *time.Time `json:"signedInAt"`
	LoginDueAt *time.Time `json:"loginDueAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	Folder     string     `json:"folder"`
	// DefaultHome is set for the account that plain claude (~/.claude) is also
	// signed in as, from the cache of the default home
	DefaultHome bool `json:"defaultHome"`
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// rowFor is what the state file says about an account. signedIn and reason are
// filled in by asking claude.
func rowFor(acc account.Account) accountRow {
	m := acc.Meta
	row := accountRow{
		ID: acc.Name, Label: shown(acc), Email: clean(m.Email), Org: clean(m.Org), Plan: clean(m.Plan),
		SignedInAt: timeOrNil(m.SignedInAt), LastUsedAt: timeOrNil(m.LastUsedAt),
	}
	// no path for an id that is not safe to use as one
	if folder, err := folderFor(acc); err == nil {
		row.Folder = folder
	}
	if !m.SignedInAt.IsZero() {
		due := loginDue(m.SignedInAt)
		row.LoginDueAt = &due
	}
	return row
}

// signInOf asks claude whether an account can be used. nil means it could not
// be checked.
func signInOf(acc account.Account) (signedIn *bool, reason string) {
	folder, err := folderFor(acc)
	if err != nil {
		return nil, clean(err.Error())
	}
	st, err := liveStatus(folder)
	if err != nil {
		return nil, clean(err.Error())
	}
	ok, _, text := checkSignIn(st, folder, acc.Meta)
	return &ok, clean(text)
}

// parallelDo runs fn for 0..n-1, at most limit at a time, and waits for all.
func parallelDo(n, limit int, fn func(i int)) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

// visibleAccounts are the accounts that were not removed, by label.
func (a *App) visibleAccounts() ([]account.Account, error) {
	return a.accountsWhere(func(acc account.Account) bool { return !acc.Meta.Hidden })
}

// removedAccounts are the accounts that were removed, by label.
func (a *App) removedAccounts() ([]account.Account, error) {
	return a.accountsWhere(func(acc account.Account) bool { return acc.Meta.Hidden })
}

// accountsWhere are the accounts that keep returns true for, by label and then
// by id.
func (a *App) accountsWhere(keep func(account.Account) bool) ([]account.Account, error) {
	all, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	var accts []account.Account
	for _, acc := range all {
		if keep(acc) {
			accts = append(accts, acc)
		}
	}
	sort.Slice(accts, func(i, j int) bool {
		li, lj := strings.ToLower(accts[i].Label()), strings.ToLower(accts[j].Label())
		if li != lj {
			return li < lj
		}
		return accts[i].Name < accts[j].Name
	})
	return accts, nil
}

// visibleRows are the visible accounts, each with its live sign-in state.
func (a *App) visibleRows() ([]accountRow, error) {
	accts, err := a.visibleAccounts()
	if err != nil {
		return nil, err
	}
	rows := make([]accountRow, len(accts))
	parallelDo(len(accts), statusWorkers, func(i int) {
		rows[i] = rowFor(accts[i])
		rows[i].SignedIn, rows[i].Reason = signInOf(accts[i])
	})
	if def := a.defaultHomeAccount(); def != "" {
		for i := range rows {
			rows[i].DefaultHome = rows[i].ID == def
		}
	}
	return rows, nil
}

func (a *App) cmdList(args []string) error {
	if len(args) > 0 {
		return usageError{"list takes no arguments", "clalt list [--json]"}
	}
	rows, err := a.visibleRows()
	if err != nil {
		return err
	}
	if a.JSON {
		return a.emitJSON(rows)
	}
	if len(rows) == 0 {
		a.printf("no accounts yet. sign one in with: clalt login <name>\n")
		return nil
	}
	now := a.clock()
	tw := tabwriter.NewWriter(a.out(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "LABEL\tEMAIL\tPLAN\tSIGNED IN\tLOGIN DUE\tLAST USED")
	for _, r := range rows {
		last := ago(derefTime(r.LastUsedAt), now)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Label, dash(r.Email), dash(r.Plan), signedInCell(r), dueCell(r, now), last)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, r := range rows {
		if r.DefaultHome {
			a.printf("\n%s is also the default claude (~/.claude)\n", r.Label)
		}
	}
	return nil
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func signedInCell(r accountRow) string {
	switch {
	case r.SignedIn == nil:
		return "unknown: " + r.Reason
	case *r.SignedIn:
		return "yes"
	}
	return "no: " + r.Reason
}

func dueCell(r accountRow, now time.Time) string {
	if r.LoginDueAt == nil {
		return "-"
	}
	if r.LoginDueAt.Before(now) {
		return dateOf(*r.LoginDueAt) + " (overdue)"
	}
	return dateOf(*r.LoginDueAt)
}

// ago words how long ago t was, for a column that has to stay short.
func ago(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return dateOf(t)
}
