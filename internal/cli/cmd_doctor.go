package cli

import (
	"os"
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/signing"
)

// doctorReport is the machine-readable diagnosis.
type doctorReport struct {
	LiveInSync   bool     `json:"liveInSync"`
	LiveFile     bool     `json:"liveFilePresent"`
	LiveKeychain bool     `json:"liveKeychainPresent"`
	Active       string   `json:"active"`
	NearExpiry   []string `json:"nearExpiry"`
	Orphans      []string `json:"orphanedSecrets"`
	MissingKeys  []string `json:"metadataWithoutSecret"`
	Signing      string   `json:"codeSigning"` // stable | adhoc | unsigned | unknown
	Fixed        []string `json:"fixed,omitempty"`
}

func (a *App) cmdDoctor(args []string) error {
	fix := false
	for _, arg := range args {
		if arg == "--fix" {
			fix = true
		}
	}

	rep := doctorReport{}
	sync, err := a.Live.SyncStatus()
	if err != nil {
		return err
	}
	rep.LiveInSync = sync.InSync
	rep.LiveFile = sync.FilePresent
	rep.LiveKeychain = sync.KeyPresent
	rep.Active, _ = a.Store.Active()
	// code signing only affects macOS Keychain access; off macOS it is moot
	// (the live credential is a plain file), so report n/a there.
	if runtime.GOOS != "darwin" {
		rep.Signing = "n/a"
	} else if exe, eerr := os.Executable(); eerr == nil {
		// how the running binary is signed decides whether keychain "always
		// allow" grants persist; ad-hoc means the password prompt recurs.
		rep.Signing = signing.Running(exe).String()
	} else {
		rep.Signing = signing.Unknown.String()
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
	// secrets in the keychain namespace with no metadata entry
	if secretNames, lerr := a.KC.List(account.KeychainService); lerr == nil {
		for _, n := range secretNames {
			if !metaNames[n] {
				rep.Orphans = append(rep.Orphans, n)
			}
		}
	}

	if fix {
		// re-sync the live pair: read freshest and write both sides
		if cred, rerr := a.Live.Read(); rerr == nil {
			if werr := a.Live.Write(cred); werr == nil {
				rep.Fixed = append(rep.Fixed, "resynced live keychain/file")
			}
		}
		// refresh a near-expiry active account
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
	a.printf("live store sync:  %s\n", okBad(rep.LiveInSync))
	a.printf("  file present:   %v\n", rep.LiveFile)
	a.printf("  keychain:       %v\n", rep.LiveKeychain)
	a.printf("active account:   %s\n", dash(rep.Active))
	a.printf("near expiry:      %s\n", listOrNone(rep.NearExpiry))
	a.printf("orphaned secrets: %s\n", listOrNone(rep.Orphans))
	a.printf("meta w/o secret:  %s\n", listOrNone(rep.MissingKeys))
	a.printf("code signing:     %s\n", rep.Signing)
	for _, f := range rep.Fixed {
		a.printf("fixed: %s\n", f)
	}
	if rep.Signing == "adhoc" || rep.Signing == "unsigned" {
		a.printf("\nclalt is %s-signed, so macOS re-prompts for your login password every run.\n", rep.Signing)
		a.printf("build with a stable identity to make 'always allow' stick:\n")
		a.printf("  make install CODESIGN_IDENTITY=clalt-codesign\n")
		a.printf("see the 'macOS Keychain prompts' section of the README for the one-time setup.\n")
	}
	if len(rep.Fixed) == 0 && (!rep.LiveInSync || len(rep.NearExpiry) > 0) {
		a.printf("\nrun 'clalt doctor --fix' to resync and refresh.\n")
	}
}

func okBad(ok bool) string {
	if ok {
		return "OK"
	}
	return "MISMATCH"
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
