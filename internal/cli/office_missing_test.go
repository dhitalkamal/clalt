package cli

import (
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/dhitalkamal/clalt/internal/reviews"
)

// fakeInfo is an executable regular file for the fake Stat
type fakeInfo struct{ os.FileInfo }

func (fakeInfo) Mode() os.FileMode { return 0o755 }

func missingResolver(found map[string]bool) reviews.Resolver {
	return reviews.Resolver{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Stat: func(p string) (os.FileInfo, error) {
			if found[p] {
				return fakeInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		Home: "/home/x",
	}
}

func TestMissingToolsReportsBoth(t *testing.T) {
	r := missingResolver(nil)
	got := missingTools(r, "claude")
	if len(got) != 2 || got[0].Name != "gh" || got[1].Name != "claude" {
		t.Fatalf("got %+v", got)
	}
	for _, m := range got {
		if !reflect.DeepEqual(m.Dirs, r.Dirs()) {
			t.Errorf("%s dirs %v, want %v", m.Name, m.Dirs, r.Dirs())
		}
	}
}

func TestMissingToolsGHFoundInFallbackDir(t *testing.T) {
	r := missingResolver(map[string]bool{"/opt/homebrew/bin/gh": true})
	got := missingTools(r, "claude")
	if len(got) != 1 || got[0].Name != "claude" {
		t.Errorf("got %+v", got)
	}
}

func TestMissingToolsNoneMissing(t *testing.T) {
	r := missingResolver(map[string]bool{"/opt/homebrew/bin/gh": true, "/usr/local/bin/claude": true})
	if got := missingTools(r, "claude"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestMissingToolsSkipsAbsoluteClaude(t *testing.T) {
	r := missingResolver(map[string]bool{"/opt/homebrew/bin/gh": true})
	if got := missingTools(r, "/nonexistent/claude"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}
