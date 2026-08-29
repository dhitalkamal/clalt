package cli

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/oauth"
)

// accountView is the JSON shape for list/whoami.
type accountView struct {
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	Email     string    `json:"email,omitempty"`
	Plan      string    `json:"plan,omitempty"`
	AddedAt   time.Time `json:"addedAt"`
	LastUsed  time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

func (a *App) cmdList() error {
	accts, err := a.Store.List()
	if err != nil {
		return err
	}
	views := make([]accountView, 0, len(accts))
	for _, ac := range accts {
		v := accountView{
			Name: ac.Name, Active: ac.Active, Email: ac.Meta.Email,
			Plan: ac.Meta.Plan, AddedAt: ac.Meta.AddedAt, LastUsed: ac.Meta.LastUsedAt,
		}
		if c, _, gerr := a.Store.Get(ac.Name); gerr == nil {
			v.ExpiresAt = c.ExpiresAtTime()
		}
		views = append(views, v)
	}
	if a.JSON {
		return a.emitJSON(views)
	}
	if len(views) == 0 {
		a.printf("no accounts yet. run 'clalt add <name>' or 'clalt login <name>'.\n")
		return nil
	}
	tw := tabwriter.NewWriter(a.Out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "\tNAME\tPLAN\tEMAIL\tTOKEN")
	for _, v := range views {
		marker := " "
		if v.Active {
			marker = "*"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", marker, v.Name, dash(v.Plan), dash(v.Email), humanUntil(v.ExpiresAt))
	}
	return tw.Flush()
}

func (a *App) cmdWhoami() error {
	name, err := a.Store.Active()
	if err != nil {
		return err
	}
	if name == "" {
		if a.JSON {
			return a.emitJSON(map[string]any{"active": nil})
		}
		a.printf("no active account\n")
		return nil
	}
	c, meta, err := a.Store.Get(name)
	if err != nil {
		return err
	}
	if a.JSON {
		return a.emitJSON(accountView{
			Name: name, Active: true, Email: meta.Email, Plan: meta.Plan,
			AddedAt: meta.AddedAt, LastUsed: meta.LastUsedAt, ExpiresAt: c.ExpiresAtTime(),
		})
	}
	a.printf("%s (%s)\n", name, dash(meta.Plan))
	if meta.Email != "" {
		a.printf("  email:  %s\n", meta.Email)
	}
	a.printf("  token:  %s\n", humanUntil(c.ExpiresAtTime()))
	return nil
}

func (a *App) cmdAdd(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: clalt add <name>")
	}
	name := args[0]
	cred, err := a.Live.Read()
	if err != nil {
		return fmt.Errorf("no current Claude Code login to capture: %w", err)
	}
	meta := a.labelMeta(cred)
	if err := a.Store.Save(name, cred, meta); err != nil {
		return err
	}
	if err := a.Store.SetActive(name); err != nil {
		return err
	}
	a.printf("captured current login as %q%s\n", name, emailSuffix(meta.Email))
	return nil
}

func (a *App) cmdRemove(args []string) error {
	name, force := "", false
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
			continue
		}
		name = arg
	}
	if name == "" {
		return errors.New("usage: clalt remove <name> [--force]")
	}
	if !force && !a.confirm(fmt.Sprintf("delete account %q?", name)) {
		a.printf("aborted\n")
		return nil
	}
	if err := a.Store.Remove(name); err != nil {
		return err
	}
	a.printf("removed %q\n", name)
	return nil
}

func (a *App) cmdRename(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: clalt rename <old> <new>")
	}
	if err := a.Store.Rename(args[0], args[1]); err != nil {
		return err
	}
	a.printf("renamed %q to %q\n", args[0], args[1])
	return nil
}

// labelMeta builds metadata for a credential, best-effort filling email from the
// profile endpoint (non-fatal) and plan from the token's subscriptionType.
func (a *App) labelMeta(cred credential.Credential) account.Meta {
	meta := account.Meta{Plan: cred.SubscriptionType}
	if a.ApiBase == "" {
		return meta // profile lookup disabled (e.g. tests)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if p, err := oauth.FetchProfile(ctx, a.OAuth.HTTP, a.ApiBase, cred.AccessToken); err == nil {
		meta.Email = p.Email
	}
	return meta
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func emailSuffix(email string) string {
	if email == "" {
		return ""
	}
	return " (" + email + ")"
}
