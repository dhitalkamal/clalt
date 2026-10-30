package cli

import (
	"github.com/dhitalkamal/clalt/internal/officed"
	"github.com/dhitalkamal/clalt/internal/reviews"
)

// missingTools lists the review tools (gh, then claudeName) the resolver
// cannot find.
func missingTools(r reviews.Resolver, claudeName string) []officed.MissingTool {
	var out []officed.MissingTool
	for _, name := range []string{"gh", claudeName} {
		if _, err := r.Resolve(name); err != nil {
			out = append(out, officed.MissingTool{Name: name, Dirs: r.Dirs()})
		}
	}
	return out
}
