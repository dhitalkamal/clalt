package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

func linksCheck(label, folder string) checkResult {
	shared := config.SharedClaudeDir()
	steps, err := planLinks(folder)
	if errors.Is(err, profile.ErrNoSharedHome) {
		return result(label, "links", statusWarn, "%v", noSharedHome(shared))
	}
	if err != nil {
		return result(label, "links", statusWarn, "could not check the links: %v", err)
	}
	var fix []string
	var kept []profile.Step
	var projects *profile.Step
	for i, st := range steps {
		switch {
		case st.Name == "projects":
			projects = &steps[i]
			if st.Action != profile.Keep {
				fix = append(fix, st.Name)
			}
		case st.Action != profile.Keep:
			fix = append(fix, st.Name)
		default:
			kept = append(kept, st)
		}
	}
	// the planner has nothing to say about a projects link that was never made
	// because the shared folder is on an unmounted volume, and only keeps one that
	// is there, so the folder is looked at the way run does. a link or a folder
	// the plan fixes is left to the plan.
	var history string
	if projects == nil || projects.Action == profile.Keep {
		switch historyOf(folder) {
		case historyOwn:
			why := ""
			if projects != nil {
				why = " (" + projects.Why + ")"
			}
			return result(label, "links", statusFail,
				"projects is not linked to the shared history%s. move what is in it into %s, remove it and run 'clalt run %s'",
				why, filepath.Join(shared, "projects"), label)
		case historyUnreachable:
			history = fmt.Sprintf("the shared history folder %s is not reachable (an unmounted volume?), or projects aims somewhere else, so 'clalt run %s' refuses",
				filepath.Join(shared, "projects"), label)
		}
	}
	if len(steps) == 0 && history == "" {
		return result(label, "links", statusOK, "in sync with the shared claude home")
	}
	var parts []string
	if len(fix) > 0 {
		parts = append(parts, fmt.Sprintf("%s to fix, 'clalt run %s' does it (%s)", count(len(fix), "link"), label, strings.Join(fix, ", ")))
	}
	if len(kept) > 0 {
		parts = append(parts, "left as they are: "+keptList(kept))
	}
	if history != "" {
		parts = append(parts, history)
	}
	return result(label, "links", statusWarn, "out of sync: %s", strings.Join(parts, "; "))
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
