package profile

import "testing"

func TestShareable(t *testing.T) {
	cases := []struct {
		name  string
		isDir bool
		want  bool
	}{
		// folders are linked unless claude keeps live state in them
		{"agents", true, true},
		{"commands", true, true},
		{"projects", true, true},
		// split folders are linked entry by entry, never whole
		{"skills", true, false},
		{"plugins", true, false},
		{"a-folder-added-next-week", true, true},
		{"sessions", true, false},
		{"daemon", true, false},
		{"jobs", true, false},
		{"backups", true, false},
		{"cache", true, false},
		{"statsig", true, false},
		{"telemetry", true, false},
		{"downloads", true, false},
		// loose files only from the allow-list
		{"settings.json", false, true},
		{"settings.local.json", false, true},
		{"CLAUDE.md", false, true},
		{"keybindings.json", false, true},
		{"history.jsonl", false, true},
		{"stats-cache.json", false, false},
		{"mcp-needs-auth-cache.json", false, false},
		{".DS_Store", false, false},
		// a file named like a per-account folder is still a loose file
		{"cache", false, false},
		// never, whatever the kind
		{".credentials.json", false, false},
		{".credentials.json", true, false},
		{".claude.json", false, false},
		{".claude.json", true, false},
	}
	for _, c := range cases {
		if got := shareable(c.name, c.isDir); got != c.want {
			t.Errorf("shareable(%q, dir=%v) = %v, want %v", c.name, c.isDir, got, c.want)
		}
	}
}

// the never-link list wins even if someone adds a secret to an allow-list.
func TestShareableNeverLinkBeatsAllowList(t *testing.T) {
	for _, name := range []string{".credentials.json", ".claude.json"} {
		sharedFiles[name] = true
		t.Cleanup(func() { delete(sharedFiles, name) })
		if shareable(name, false) {
			t.Errorf("shareable(%q) = true after adding it to the allow-list", name)
		}
	}
}

func TestSplitDirsKeepTheNamesClaudeAiWritesPerAccount(t *testing.T) {
	want := map[string][]string{
		"skills":  {".trash", "synced"},
		"plugins": {".last_inuse_sweep", ".trash", "known_marketplaces_claudeai.json", "plugin-directory-cache-v2.json", "synced"},
	}
	if len(splitDirs) != len(want) {
		t.Errorf("split folders = %v, want %v", splitDirs, want)
	}
	for dir, names := range want {
		if len(splitDirs[dir]) != len(names) {
			t.Errorf("%s keeps %v per account, want %v", dir, splitDirs[dir], names)
		}
		for _, n := range names {
			if !splitDirs[dir][n] {
				t.Errorf("%s/%s is not kept per account", dir, n)
			}
		}
	}
}
