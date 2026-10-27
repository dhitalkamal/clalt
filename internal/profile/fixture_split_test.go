package profile

import (
	"path/filepath"
	"testing"
)

// splitShared adds skills and plugins folders to a shared fixture, with the
// entries claude keeps per account among them (synced, .trash and the claude.ai
// marketplace list).
func splitShared(t *testing.T, shared string) {
	t.Helper()
	for _, d := range []string{
		"skills/review", "skills/deploy", "skills/synced", "skills/.trash",
		"plugins/cache", "plugins/marketplaces", "plugins/synced", "plugins/.trash",
	} {
		mkdirT(t, filepath.Join(shared, d))
	}
	for _, f := range []string{
		"skills/README.md", "plugins/installed_plugins.json",
		"plugins/known_marketplaces.json", "plugins/known_marketplaces_claudeai.json",
	} {
		writeT(t, filepath.Join(shared, f), "x")
	}
}

// what a fresh account links out of the folders above, sorted.
var (
	skillsLinked  = []string{"skills/README.md", "skills/deploy", "skills/review"}
	pluginsLinked = []string{"plugins/cache", "plugins/installed_plugins.json", "plugins/known_marketplaces.json", "plugins/marketplaces"}
)

func linkSplit(t *testing.T, acct, shared string) {
	t.Helper()
	for _, n := range append(append([]string{}, skillsLinked...), pluginsLinked...) {
		mustSymlink(t, filepath.Join(shared, filepath.FromSlash(n)), filepath.Join(acct, filepath.FromSlash(n)))
	}
}
