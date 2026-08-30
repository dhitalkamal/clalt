package cli

import (
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
)

// doctorReport is the machine-readable diagnosis. clalt is keychain-free, so
// there is no live keychain/file sync or code-signing state to report anymore.
type doctorReport struct {
	Active      string   `json:"active"`
	LoginFile   bool     `json:"loginFilePresent"` // a live Claude login `add` could capture
	NearExpiry  []string `json:"nearExpiry"`
	Orphans     []string `json:"orphanedSecrets"`
	MissingKeys []string `json:"metadataWithoutSecret"`
	Fixed       []string `json:"fixed,omitempty"`
}

func (a *App) cmdDoctor(args []string) error {
	fix := false
	for _, arg := range args {
		if arg == "--fix" {
			fix = true
		}
	}

	rep := doctorReport{}
	rep.Active, _ = a.Store.Active()
	if cred, err := a.Live.Read(); err == nil && cred.AccessToken != "" {
		rep.LoginFile = true
	}

	accts, err := a.Store.List()
	if err != nil {
		return err
	}
	metaNames := map[string]bool{}
	for _, ac := range accts {
		metaNames[ac.Name] = true
		c, _, gerr := a.Store.Get(ac.Name)
		if gerr != nil {
			rep.MissingKeys = append(rep.MissingKeys, ac.Name)
			continue
		}
		if c.IsExpired(5 * time.Minute) {
			rep.NearExpiry = append(rep.NearExpiry, ac.Name)
		}
	}
	// vault secrets with no metadata entry
	if secretNames, lerr := a.KC.List(account.KeychainService); lerr == nil {
		for _, n := range secretNames {
			if !metaNames[n] {
				rep.Orphans = append(rep.Orphans, n)
			}
		}
	}

	if fix {
		// refresh a near-expiry active account so the next `run` starts fresh
		if rep.Active != "" && contains(rep.NearExpiry, rep.Active) {
			if err := a.cmdRefresh([]string{rep.Active}); err == nil {
				rep.Fixed = append(rep.Fixed, "refreshed active token")
			}
		}
	}

	if a.JSON {
		return a.emitJSON(rep)
	}
	a.renderDoctor(rep)
	return nil
}

func (a *App) renderDoctor(rep doctorReport) {
	a.printf("active account:   %s\n", dash(rep.Active))
	a.printf("login file:       %v\n", rep.LoginFile)
	a.printf("near expiry:      %s\n", listOrNone(rep.NearExpiry))
	a.printf("orphaned secrets: %s\n", listOrNone(rep.Orphans))
	a.printf("meta w/o secret:  %s\n", listOrNone(rep.MissingKeys))
	for _, f := range rep.Fixed {
		a.printf("fixed: %s\n", f)
	}
	if len(rep.Fixed) == 0 && len(rep.NearExpiry) > 0 {
		a.printf("\nrun 'clalt doctor --fix' to refresh near-expiry tokens.\n")
	}
}

func listOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
