package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// whoami says where this shell is: inside an account's folder, which is what a
// `clalt run` session looks like, inside an old v0.1.0 profile, or nowhere
// clalt knows. the answer comes from CLAUDE_CONFIG_DIR, and for an account
// from asking claude too.
func (a *App) cmdWhoami(args []string) error {
	if len(args) > 0 {
		return usageError{"whoami takes no arguments", "clalt whoami [--json]"}
	}
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if id, ok := insideFolder(config.AccountsDir(), dir); ok {
		return a.whoamiAccount(id, dir)
	}
	if _, ok := insideFolder(config.ProfilesDir(), dir); ok {
		if a.JSON {
			return a.emitJSON(map[string]string{"kind": "oldProfile", "folder": dir})
		}
		a.printf("this shell is in an old v0.1.0 profile session (%s).\n", clean(dir))
		a.printf("'clalt migrate' copies its history. sign in with 'clalt login <name>' and start sessions with 'clalt run <name>'.\n")
		return nil
	}
	if a.JSON {
		return a.emitJSON(map[string]string{"kind": "none"})
	}
	a.printf("this shell is not in a clalt account: plain claude uses the normal login.\n")
	return nil
}

// insideFolder reports the first folder name below root on the way to dir, if
// dir is below root. paths are compared after resolving links, because a temp
// dir on macOS is spelled /var and lives in /private/var.
func insideFolder(root, dir string) (first string, ok bool) {
	if dir == "" {
		return "", false
	}
	rel, err := filepath.Rel(fsutil.ResolvePath(root), fsutil.ResolvePath(dir))
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return "", false
	}
	first, _, _ = strings.Cut(rel, string(filepath.Separator))
	return first, true
}

func (a *App) whoamiAccount(id, dir string) error {
	all, err := a.Store.List()
	if err != nil {
		return err
	}
	for _, acc := range all {
		if acc.Name != id {
			continue
		}
		if acc.Meta.Hidden {
			if a.JSON {
				return a.emitJSON(map[string]string{"kind": "removedAccount", "id": id, "label": shown(acc), "folder": dir})
			}
			a.printf("this shell is in the folder of %s, which was removed. 'clalt login %s' brings it back.\n", shown(acc), shown(acc))
			return nil
		}
		row := rowFor(acc)
		row.SignedIn, row.Reason = signInOf(acc)
		if a.JSON {
			return a.emitJSON(struct {
				Kind string `json:"kind"`
				accountRow
			}{"account", row})
		}
		a.printWhoami(row)
		return nil
	}
	if a.JSON {
		return a.emitJSON(map[string]string{"kind": "unknownAccount", "id": id, "folder": dir})
	}
	a.printf("this shell is in %s, an account folder clalt does not know. run 'clalt list' to see the accounts it does.\n", clean(dir))
	return nil
}

func (a *App) printWhoami(r accountRow) {
	a.printf("%s%s\n", r.Label, parenthesized(r.Plan, r.Org))
	field := func(name, value string) { a.printf("  %-17s %s\n", name+":", value) }
	if r.Email != "" {
		field("email", r.Email)
	}
	switch {
	case r.SignedIn == nil:
		field("signed in", "unknown ("+r.Reason+")")
	case *r.SignedIn:
		field("signed in", "yes")
	default:
		field("signed in", "no, "+r.Reason+". sign in with: clalt login "+r.Label)
	}
	if r.LoginDueAt != nil {
		field("sign in again by", dateOf(*r.LoginDueAt))
	}
	field("folder", clean(r.Folder))
}
