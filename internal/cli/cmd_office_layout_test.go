package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// officeLayoutFile writes a small valid layout (a lobby only, so no catalog is
// needed) under a name and returns its path.
func officeLayoutFile(t *testing.T, dir, file, name string) string {
	t.Helper()
	l := layout.Minimal()
	l.Name, l.Title = name, "Title of "+name
	raw, err := layout.Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, file)
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOfficeLayoutImportListUseExportReset(t *testing.T) {
	e := newEnv(t)
	src := officeLayoutFile(t, t.TempDir(), "any-file-name.json", "mine")

	if code := e.run("office", "layout", "import", src); code != 0 {
		t.Fatalf("import = %d %s", code, e.errOut)
	}
	if _, err := os.Stat(filepath.Join(config.OfficeDir(), "layouts", "mine.json")); err != nil {
		t.Fatal(err)
	}
	e.reset()
	if e.run("office", "layout", "list") != 0 || !strings.Contains(e.out.String(), "mine") || !strings.Contains(e.out.String(), "Title of mine") {
		t.Errorf("list: %q", e.out)
	}
	if e.run("office", "layout", "use", "mine") != 0 {
		t.Fatalf("use: %s", e.errOut)
	}
	if got := layout.NewStore(config.OfficeDir(), nil).ActiveName(); got != "mine" {
		t.Errorf("active = %q", got)
	}
	e.reset()
	e.run("office", "layout", "list")
	if !strings.Contains(e.out.String(), "* mine") {
		t.Errorf("list does not mark the active layout: %q", e.out)
	}

	e.reset()
	if e.run("office", "layout", "export", "mine") != 0 || !strings.Contains(e.out.String(), `"name": "mine"`) {
		t.Errorf("export to stdout: %q %q", e.out, e.errOut)
	}
	out := filepath.Join(t.TempDir(), "out.json")
	if e.run("office", "layout", "export", "mine", out) != 0 {
		t.Fatalf("export to a file: %s", e.errOut)
	}
	e.reset()
	if code := e.run("office", "layout", "export", "mine", out); code == 0 || !strings.Contains(e.errOut.String(), "--force") {
		t.Errorf("overwrote a file without --force: %d %q", code, e.errOut)
	}
	if e.run("office", "layout", "export", "mine", out, "--force") != 0 {
		t.Errorf("--force export failed: %s", e.errOut)
	}

	if e.run("office", "layout", "reset") != 0 {
		t.Fatal(e.errOut.String())
	}
	if got := layout.NewStore(config.OfficeDir(), nil).ActiveName(); got != "default" {
		t.Errorf("active after reset = %q", got)
	}
	if _, err := os.Stat(filepath.Join(config.OfficeDir(), "layouts", "mine.json")); err != nil {
		t.Errorf("reset deleted a layout: %v", err)
	}
}

func TestOfficeLayoutImportWithAName(t *testing.T) {
	e := newEnv(t)
	src := officeLayoutFile(t, t.TempDir(), "x.json", "mine")
	if e.run("office", "layout", "import", src, "--name", "renamed") != 0 {
		t.Fatal(e.errOut.String())
	}
	got, err := layout.NewStore(config.OfficeDir(), nil).Load("renamed")
	if err != nil || got.Name != "renamed" {
		t.Errorf("%v %+v", err, got.Name)
	}
	if e.run("office", "layout", "import", src, "--name", "../bad") == 0 {
		t.Error("imported under a path name")
	}
}

func TestOfficeLayoutImportRefusals(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := officeLayoutFile(t, dir, "good.json", "good")
	cases := map[string][]string{
		"missing file":   {"import", filepath.Join(dir, "nope.json")},
		"not json":       {"import", write("a.json", "nope")},
		"unknown key":    {"import", write("b.json", `{"v":1,"evil":"x"}`)},
		"oversize":       {"import", write("c.json", strings.Repeat(" ", layout.MaxBytes+1))},
		"built-in name":  {"import", good, "--name", "default"},
		"no file":        {"import"},
		"extra argument": {"import", good, "extra"},
		"name no value":  {"import", good, "--name"},
	}
	for name, args := range cases {
		e := newEnv(t)
		e.app.Out, e.app.Err = e.out, e.errOut
		if code := e.run(append([]string{"office", "layout"}, args...)...); code == 0 {
			t.Errorf("%s: exit 0", name)
		}
		if e.errOut.Len() == 0 {
			t.Errorf("%s: no message", name)
		}
		if ents, _ := os.ReadDir(filepath.Join(config.OfficeDir(), "layouts")); len(ents) != 0 {
			t.Errorf("%s: left %v behind", name, ents)
		}
	}
	// a second import of a name that exists needs --force
	e := newEnv(t)
	if e.run("office", "layout", "import", good) != 0 {
		t.Fatal(e.errOut.String())
	}
	if e.run("office", "layout", "import", good) == 0 {
		t.Error("overwrote a layout without --force")
	}
	if e.run("office", "layout", "import", good, "--force") != 0 {
		t.Error(e.errOut.String())
	}
}

func TestOfficeLayoutUseAndExportRefuseUnknownNames(t *testing.T) {
	e := newEnv(t)
	for _, args := range [][]string{{"use", "nope"}, {"use", "../x"}, {"use"}, {"export", "nope"}, {"export"}, {"bogus"}, {}, {"reset", "x"}, {"list", "x"}} {
		e.reset()
		if code := e.run(append([]string{"office", "layout"}, args...)...); code == 0 {
			t.Errorf("%v: exit 0", args)
		}
	}
}

func TestOfficeEditFlag(t *testing.T) {
	for _, edit := range []bool{false, true} {
		e := newEnv(t)
		args := []string{"--no-open"}
		if edit {
			args = append(args, "--edit")
		}
		url, stop := startOffice(t, e, func(string) error { return nil }, args...)
		base, token, _ := strings.Cut(url, "/?t=")
		code, body := fetch(t, base+"/api/layout", token)
		want := `"edit":false`
		if edit {
			want = `"edit":true`
		}
		if code != 200 || !strings.Contains(body, want) || !strings.Contains(body, `"name":"default"`) {
			t.Errorf("edit=%v: %d %s", edit, code, body)
		}
		req, _ := http.NewRequest("GET", base+"/api/layouts", nil)
		req.Header.Set("X-Clalt-Token", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if (resp.StatusCode == 200) != edit {
			t.Errorf("edit=%v: GET /api/layouts = %d", edit, resp.StatusCode)
		}
		stop()
	}
}
