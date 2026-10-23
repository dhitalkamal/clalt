package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// sharedEnv points HOME and XDG_CONFIG_HOME into a temp dir and clears
// CLAUDE_CONFIG_DIR, so nothing here can see the real claude home.
func sharedEnv(t *testing.T) (home, xdg string) {
	t.Helper()
	root := t.TempDir()
	home = filepath.Join(root, "home")
	xdg = filepath.Join(root, "xdg")
	for _, d := range []string{home, xdg} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home, xdg
}

func TestSharedHomeDefaultsToDotClaude(t *testing.T) {
	home, _ := sharedEnv(t)
	if got, want := SharedClaudeDir(), filepath.Join(home, ".claude"); got != want {
		t.Errorf("SharedClaudeDir() = %q, want %q", got, want)
	}
	if got, want := SharedClaudeJSONPath(), filepath.Join(home, ".claude.json"); got != want {
		t.Errorf("SharedClaudeJSONPath() = %q, want %q", got, want)
	}
}

func TestSharedHomeFollowsCustomConfigDir(t *testing.T) {
	home, _ := sharedEnv(t)
	custom := filepath.Join(filepath.Dir(home), "elsewhere", "claude")
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	if got := SharedClaudeDir(); got != custom {
		t.Errorf("SharedClaudeDir() = %q, want %q", got, custom)
	}
	// with a custom dir the global config moves inside it
	if got, want := SharedClaudeJSONPath(), filepath.Join(custom, ".claude.json"); got != want {
		t.Errorf("SharedClaudeJSONPath() = %q, want %q", got, want)
	}
}

func TestSharedHomeIgnoresClalt(t *testing.T) {
	home, _ := sharedEnv(t)
	sep := string(filepath.Separator)
	cases := map[string]string{
		"account folder":     accountDir("robin"),
		"folder not created": filepath.Join(AccountsDir(), "never-made", "deeper"),
		"clalt dir itself":   ClaltDir(),
		"dotdot spelling":    AccountsDir() + sep + "x" + sep + ".." + sep + "robin" + sep,
	}
	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", dir)
			if got, want := SharedClaudeDir(), filepath.Join(home, ".claude"); got != want {
				t.Errorf("SharedClaudeDir() = %q, want %q", got, want)
			}
			if got, want := SharedClaudeJSONPath(), filepath.Join(home, ".claude.json"); got != want {
				t.Errorf("SharedClaudeJSONPath() = %q, want %q", got, want)
			}
		})
	}
}

func TestSharedHomeComparesAcrossSymlinks(t *testing.T) {
	home, xdg := sharedEnv(t)
	// clalt's dir is reached through a link, the session dir through the real path
	link := filepath.Join(filepath.Dir(xdg), "xdg-link")
	if err := os.Symlink(xdg, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", link)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(xdg, "clalt", "accounts", "robin"))
	if got, want := SharedClaudeDir(), filepath.Join(home, ".claude"); got != want {
		t.Errorf("SharedClaudeDir() = %q, want %q", got, want)
	}
}

func TestSharedHomeSiblingOfClaltIsOutside(t *testing.T) {
	_, xdg := sharedEnv(t)
	sibling := filepath.Join(xdg, "clalt-backup")
	t.Setenv("CLAUDE_CONFIG_DIR", sibling)
	if got := SharedClaudeDir(); got != sibling {
		t.Errorf("SharedClaudeDir() = %q, want %q (a shared prefix is not inside clalt)", got, sibling)
	}
}

func TestSharedHomeIsAbsoluteAndClean(t *testing.T) {
	home, _ := sharedEnv(t)
	root := filepath.Dir(home)
	t.Chdir(root)
	t.Setenv("CLAUDE_CONFIG_DIR", "rel/dir/")
	got := SharedClaudeDir()
	if !filepath.IsAbs(got) || got != filepath.Clean(got) {
		t.Errorf("SharedClaudeDir() = %q, want an absolute cleaned path", got)
	}
	if !fsutil.SamePath(got, filepath.Join(root, "rel", "dir")) {
		t.Errorf("SharedClaudeDir() = %q, want it to lead to %s/rel/dir", got, root)
	}
}

func TestSharedHomeIsEmptyWhenThereIsNoUsableHome(t *testing.T) {
	sharedEnv(t)
	for _, home := range []string{"", "relative/home"} {
		t.Setenv("HOME", home)
		// windows asks for this one when HOME is empty
		t.Setenv("USERPROFILE", home)
		if got := SharedClaudeDir(); got != "" {
			t.Errorf("HOME=%q: SharedClaudeDir() = %q, want empty rather than a relative path", home, got)
		}
		if got := SharedClaudeJSONPath(); got != "" {
			t.Errorf("HOME=%q: SharedClaudeJSONPath() = %q, want empty", home, got)
		}
	}
}

func TestSharedHomeFromCustomDirDoesNotNeedHome(t *testing.T) {
	home, _ := sharedEnv(t)
	custom := filepath.Join(filepath.Dir(home), "elsewhere", "claude")
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	t.Setenv("HOME", "")
	if got := SharedClaudeDir(); got != custom {
		t.Errorf("SharedClaudeDir() = %q, want %q", got, custom)
	}
	if got, want := SharedClaudeJSONPath(), filepath.Join(custom, ".claude.json"); got != want {
		t.Errorf("SharedClaudeJSONPath() = %q, want %q", got, want)
	}
}

func TestSharedHomeIgnoresClaltWhateverTheCase(t *testing.T) {
	home, xdg := sharedEnv(t)
	if err := os.MkdirAll(filepath.Join(xdg, "Probe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(xdg, "probe")); err != nil {
		t.Skip("case-sensitive file system")
	}
	// clalt's dir is xdg/clalt on disk; the session was started through xdg/CLALT
	if err := os.MkdirAll(filepath.Join(xdg, "clalt", "accounts", "robin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(xdg, "CLALT", "accounts", "robin"))
	if got, want := SharedClaudeDir(), filepath.Join(home, ".claude"); got != want {
		t.Errorf("SharedClaudeDir() = %q, want %q", got, want)
	}
}
