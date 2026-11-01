package reviews

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func sample(n int, sha string) Review {
	return Review{Repo: "o/r", Number: n, Title: "T", Author: "ann", URL: "u", SHA: sha,
		StartedAt: time.Unix(1000+int64(n), 0).UTC(), EndedAt: time.Unix(2000+int64(n), 0).UTC(), Account: "work",
		Findings: []Finding{{Severity: "high", File: "a.go", Line: 3, Title: "t", Detail: "d"}}, Summary: "s", Transcript: "abc-123"}
}

func TestIDIsRepoNumberSha7(t *testing.T) {
	if got := sample(5, "0123456789abcdef").ID(); got != "o-r-5-0123456" {
		t.Fatalf("id = %s", got)
	}
}

func TestStoreSaveGetListHas(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	if err := s.Save(sample(1, "aaaaaaaa11")); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(sample(2, "bbbbbbbb22")); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(dir, "o-r-1-aaaaaaa.json"))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("file = %v, %v", fi, err)
		}
	}
	got, ok, err := s.Get("o-r-2-bbbbbbb")
	if err != nil || !ok || got.Number != 2 || got.Findings[0].File != "a.go" {
		t.Fatalf("get = %+v %v %v", got, ok, err)
	}
	list, err := s.List()
	if err != nil || len(list) != 2 || list[0].Number != 2 {
		t.Fatalf("list = %+v %v (newest first)", list, err)
	}
	if !s.Has("o/r", 1, "aaaaaaaa11") || s.Has("o/r", 1, "cccccccc33") {
		t.Fatal("has is by repo, number and sha")
	}
}

func TestStoreIgnoresConfigAndJunk(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"enabled":true}`), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "o-r-9-ccccccc.json"), []byte(`not json`), 0o600)
	_ = s.Save(sample(1, "aaaaaaaa11"))
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v %v", list, err)
	}
}

func TestStoreGetRefusesBadIDs(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, id := range []string{"../x", "config", "a/b", "", "o-r-1-zzzzzzz", "o-r-1-aaaaaaa.json"} {
		if _, ok, _ := s.Get(id); ok {
			t.Errorf("%q served", id)
		}
	}
}

func TestStoreMissingDirListsEmpty(t *testing.T) {
	list, err := (Store{Dir: filepath.Join(t.TempDir(), "none")}).List()
	if err != nil || len(list) != 0 {
		t.Fatalf("%v %v", list, err)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := LoadConfig(dir)
	if err != nil || c.Enabled || len(c.Repos) != 0 {
		t.Fatalf("default = %+v %v (off by default)", c, err)
	}
	if err := SaveConfig(dir, Config{Enabled: true, Repos: []string{"o/b", "o/a", "o/b"}}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(filepath.Join(dir, "config.json"))
		if fi == nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %v", fi)
		}
	}
	c, err = LoadConfig(dir)
	if err != nil || !c.Enabled || len(c.Repos) != 2 || c.Repos[0] != "o/a" {
		t.Fatalf("loaded = %+v %v (sorted, once each)", c, err)
	}
}

func TestSaveConfigRejectsBadRepos(t *testing.T) {
	for _, r := range []string{"--web", "a", "a/b/c", "../x/y", "a b/c"} {
		if err := SaveConfig(t.TempDir(), Config{Repos: []string{r}}); err == nil {
			t.Errorf("%q saved", r)
		}
	}
}

func TestLoadConfigBrokenFileIsOff(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0o600)
	c, err := LoadConfig(dir)
	if err == nil || c.Enabled {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestCleanReposDedupesByCase(t *testing.T) {
	got := cleanRepos([]string{"acme/widgets", "ACME/Widgets", "o/a"})
	want := []string{"acme/widgets", "o/a"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}
