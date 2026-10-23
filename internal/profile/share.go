package profile

// claude keeps live per-account state in these folders, so they are never
// linked even though every other folder is.
var perAccountDirs = map[string]bool{
	"sessions": true, "daemon": true, "jobs": true, "backups": true,
	"cache": true, "statsig": true, "telemetry": true, "downloads": true,
}

// loose files hold per-account state too (signed org policy, connector
// sign-in cache, daemon status, stats), so only these are linked.
var sharedFiles = map[string]bool{
	"settings.json": true, "settings.local.json": true, "CLAUDE.md": true,
	"keybindings.json": true, "history.jsonl": true,
}

var neverLink = map[string]bool{".credentials.json": true, ".claude.json": true}

// splitDirs are folders that are not linked whole. claude.ai sessions download
// an account's own skills and plugins into skills/synced and plugins/synced, and
// its marketplaces into plugins/known_marketplaces_claudeai.json, so a link to
// the shared folder would write one account's downloads into every account's.
// the account gets a real folder instead, with every other entry of the shared
// folder linked into it. the names listed here stay per account.
var splitDirs = map[string]map[string]bool{
	"skills": {"synced": true, ".trash": true},
	"plugins": {
		"synced": true, "known_marketplaces_claudeai.json": true, ".trash": true,
		// a regenerable cache and a timestamp that claude rewrites at every start
		"plugin-directory-cache-v2.json": true, ".last_inuse_sweep": true,
	},
}

// adoptable are the files of plugins/ that claude may save by renaming a new
// file over the link. when that happened the account holds a real file, and the
// sync adopts it instead of leaving the account to drift away from the others.
var adoptable = map[string]bool{
	"installed_plugins.json": true, "known_marketplaces.json": true, "plugin-catalog-cache.json": true,
}

// shareable reports whether an entry of the shared home may be linked into a
// profile whole. the never-link list is checked first so no other list can undo
// it, and a split folder is never linked whole.
func shareable(name string, isDir bool) bool {
	if neverLink[name] {
		return false
	}
	if isDir {
		return !perAccountDirs[name] && splitDirs[name] == nil
	}
	return sharedFiles[name]
}
