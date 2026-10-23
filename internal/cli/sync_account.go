package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// syncAccount links an account folder to the shared claude home and mirrors the
// shared mcp servers into it, and says what it did. trouble with one item is a
// warning; only a failure to sync at all is an error.
func (a *App) syncAccount(folder string) (profile.Report, error) {
	shared := config.SharedClaudeDir()
	if shared == "" {
		return profile.Report{}, errors.New(noHomeKnown)
	}
	// the home itself is never made here, it may only be unmounted. one that is
	// there but has no projects folder yet (claude ran, nothing was saved) gets
	// one, because without it the account's history would stay with the account
	if _, err := os.Stat(shared); err == nil {
		// an entry in any form counts, a link into a volume that is not mounted
		// included: it is for run to say that it is not reachable
		projects := filepath.Join(shared, "projects")
		if _, err := os.Lstat(projects); errors.Is(err, fs.ErrNotExist) {
			if err := os.MkdirAll(projects, 0o700); err != nil {
				return profile.Report{}, fmt.Errorf("create the shared projects folder: %w", err)
			}
		}
	}
	rep, err := syncLinks(folder)
	if errors.Is(err, profile.ErrNoSharedHome) {
		return rep, noSharedHome(shared)
	}
	if err != nil {
		return rep, fmt.Errorf("link %s to the shared claude home: %w", folder, err)
	}
	mcp, mcpErr := syncServers(folder)
	a.reportSync(rep, mcp, mcpErr)
	return rep, nil
}

// noHomeKnown is what is said when no absolute path for the claude home can be
// worked out, and a relative one is never used in its place.
const noHomeKnown = "cannot tell where the shared claude home is: is HOME set?"

func noSharedHome(shared string) error {
	return fmt.Errorf("the shared claude home %s does not exist, so there is nothing to link the account to. "+
		"run claude once to make it, or make the folder yourself, then try again", shared)
}

// reportSync says what the sync changed. all of it goes to stderr: run starts
// claude right after, and stdout is claude's.
func (a *App) reportSync(rep profile.Report, mcp profile.MCPChange, mcpErr error) {
	var parts []string
	say := func(n int, noun, verb string) {
		if n > 0 {
			parts = append(parts, count(n, noun)+" "+verb)
		}
	}
	// a link to a whole folder that became a folder of links is not a link that
	// now aims elsewhere
	var repointed, replaced []string
	for _, st := range rep.Repointed {
		if st.Folder {
			replaced = append(replaced, st.Name)
		} else {
			repointed = append(repointed, st.Name)
		}
	}
	say(len(rep.Linked), "item", "linked")
	say(len(repointed), "link", "repointed")
	say(len(rep.Removed), "dangling link", "removed")
	say(len(mcp.Added), "mcp server", "added")
	say(len(mcp.Updated), "mcp server", "updated")
	say(len(mcp.Removed), "mcp server", "removed")
	if len(parts) > 0 {
		fmt.Fprintf(a.Err, "synced with the shared claude home: %s\n", strings.Join(parts, ", "))
	}
	if len(replaced) > 0 {
		fmt.Fprintf(a.Err, "replaced the link to a whole folder with a folder of links: %s\n", clean(strings.Join(replaced, ", ")))
	}
	if len(mcp.Overwrote) > 0 {
		fmt.Fprintf(a.Err, "replaced the account's own mcp server with the shared one: %s\n", clean(strings.Join(mcp.Overwrote, ", ")))
	}
	if len(rep.Kept) > 0 {
		fmt.Fprintf(a.Err, "not linked, left as it is: %s (see 'clalt doctor')\n", clean(keptList(rep.Kept)))
	}
	for _, e := range rep.Errors {
		fmt.Fprintf(a.Err, "warning: could not link %s: %s\n", clean(e.Name), clean(e.Err.Error()))
	}
	if mcpErr != nil {
		fmt.Fprintf(a.Err, "warning: could not sync mcp servers: %s\n", clean(mcpErr.Error()))
	}
}

// keptList names the entries that were left as they are, each with the reason.
func keptList(steps []profile.Step) string {
	items := make([]string, len(steps))
	for i, st := range steps {
		items[i] = st.Name + " (" + st.Why + ")"
	}
	return strings.Join(items, "; ")
}
