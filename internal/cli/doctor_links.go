package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// linksCheck compares an account's links with the shared claude home, as `clalt
// run` is going to find it, and says what run would do about them. a real
// projects folder is the one finding that fails, since claude would keep the
// account's history to itself. the rest is for run to repair or for the user to
// mount, so it is a warning. a failure about projects comes first and does not
// hide the others.
func linksCheck(label, folder string) checkResult {
	steps, makes, err := planLinks(folder)
	if errors.Is(err, profile.ErrNoSharedHome) {
		return result(label, "links", statusWarn, "%v", noSharedHome(config.SharedClaudeDir()))
	}
	if err != nil {
		return result(label, "links", statusWarn, "could not check the links: %v", err)
	}
	var fix []string
	var kept []profile.Step
	var adopts []string
	var projects *profile.Step
	for i, st := range steps {
		switch {
		case st.Name == "projects":
			projects = &steps[i]
			// when run makes the shared folder, that is said with the folder
			if st.Action != profile.Keep && !makes {
				fix = append(fix, st.Name)
			}
		case st.Action == profile.Adopt:
			adopts = append(adopts, st.Name)
		case st.Action != profile.Keep:
			fix = append(fix, st.Name)
		default:
			kept = append(kept, st)
		}
	}
	finding, fails := historyFinding(label, folder, projects, makes)

	var parts []string
	if fails {
		parts = append(parts, finding)
	}
	if len(fix) > 0 {
		parts = append(parts, fmt.Sprintf("%s to fix, 'clalt run %s' does it (%s)", count(len(fix), "link"), label, strings.Join(fix, ", ")))
	}
	if len(adopts) > 0 {
		parts = append(parts, fmt.Sprintf("claude saved over the link, 'clalt run %s' will adopt the newer copy and back the older one up (%s)", label, strings.Join(adopts, ", ")))
	}
	if len(kept) > 0 {
		parts = append(parts, "left as they are: "+keptList(kept))
	}
	if finding != "" && !fails {
		parts = append(parts, finding)
	}
	switch {
	case fails:
		return result(label, "links", statusFail, "out of sync: %s", strings.Join(parts, "; "))
	case len(parts) > 0:
		return result(label, "links", statusWarn, "out of sync: %s", strings.Join(parts, "; "))
	}
	return result(label, "links", statusOK, "in sync with the shared claude home")
}

// historyFinding says what stands between the account and the shared history, as
// run will find it, or "" when nothing does. fails is true for a real projects
// folder. a link that the plan makes or replaces is counted with the links to
// fix, except when run makes the shared folder too, which is said here.
func historyFinding(label, folder string, projects *profile.Step, makes bool) (finding string, fails bool) {
	shared := filepath.Join(config.SharedClaudeDir(), "projects")
	planned := projects != nil && projects.Action != profile.Keep
	switch {
	case planned && !makes:
		return "", false
	case makes && (planned || projects == nil):
		// a link that aims at the folder that is not there yet is right once run
		// has made it, so there is nothing to look at in the folder
		return fmt.Sprintf("the shared history folder is missing. 'clalt run %s' makes %s and links the account to it", label, shared), false
	}
	// the plan has nothing to say about a projects link that was never made
	// because the shared folder is on an unmounted volume, and only keeps one that
	// is there, so the folder is looked at the way run does
	state, cause := historyOf(folder)
	switch state {
	case historyOwn:
		why := ""
		if projects != nil {
			why = " (" + projects.Why + ")"
		}
		return fmt.Sprintf("projects is not linked to the shared history%s. move what is in it into %s, remove it and run 'clalt run %s'",
			why, shared, label), true
	case historyUnknown:
		return fmt.Sprintf("could not look at the projects folder (%s), so 'clalt run %s' refuses", clean(cause.Error()), label), false
	case historyUnreachable:
		return fmt.Sprintf("the shared history folder %s is not reachable (an unmounted volume?), or projects aims somewhere else, so 'clalt run %s' refuses",
			shared, label), false
	}
	return "", false
}

func mcpCheck(label, folder string) checkResult {
	change, err := planServers(folder)
	if err != nil {
		return result(label, "mcp", statusWarn, "could not check the mcp servers: %v", err)
	}
	if change.Empty() {
		return result(label, "mcp", statusOK, "servers in sync with the shared config")
	}
	var parts []string
	for _, p := range []struct {
		names []string
		verb  string
	}{{change.Added, "to add"}, {change.Updated, "to update"}, {change.Removed, "to remove"}} {
		if len(p.names) > 0 {
			parts = append(parts, fmt.Sprintf("%s %s (%s)", count(len(p.names), "server"), p.verb, strings.Join(p.names, ", ")))
		}
	}
	return result(label, "mcp", statusWarn, "out of sync: %s. 'clalt run %s' syncs them", strings.Join(parts, "; "), label)
}
