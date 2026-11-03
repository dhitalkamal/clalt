package office

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// saveMine stores a user layout called "mine".
func saveMine(t *testing.T, e layoutEnv) {
	t.Helper()
	l := layout.Minimal()
	l.Name, l.Title = "mine", "Mine"
	if err := e.store.Save(l); err != nil {
		t.Fatal(err)
	}
}

type deleteReply struct {
	Active    string
	Layouts   []layout.Info
	Templates []struct{ ID string }
}

func TestDeleteRemovesAUserLayoutAndAnswersWithTheList(t *testing.T) {
	e := newLayoutEnv(t)
	saveMine(t, e)
	rec := e.send("POST", deletePath, `{"name":"mine"}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var r deleteReply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	for _, l := range r.Layouts {
		if l.Name == "mine" {
			t.Errorf("the list still holds mine: %+v", r.Layouts)
		}
	}
	if r.Active != layout.DefaultActive || len(r.Templates) == 0 {
		t.Errorf("reply %+v", r)
	}
	if _, err := e.store.Load("mine"); err == nil {
		t.Error("the file is still there")
	}
}

func TestDeleteRefusesEveryBuiltIn(t *testing.T) {
	e := newLayoutEnv(t)
	names := []string{"default", "loft"}
	for _, tpl := range layout.Templates {
		names = append(names, tpl.ID)
	}
	for _, n := range names {
		rec := e.send("POST", deletePath, `{"name":"`+n+`"}`)
		if rec.Code != 403 || strings.Count(strings.TrimSpace(rec.Body.String()), "\n") != 0 {
			t.Errorf("%s = %d %q, want 403 and one line", n, rec.Code, rec.Body)
		}
	}
	if e.store.ActiveName() != layout.DefaultActive {
		t.Errorf("a refused delete changed active to %q", e.store.ActiveName())
	}
	if _, err := e.store.Load("loft"); err != nil {
		t.Errorf("built-in gone: %v", err)
	}
}

func TestDeleteUnknownAndBadNames(t *testing.T) {
	e := newLayoutEnv(t)
	if rec := e.send("POST", deletePath, `{"name":"nope"}`); rec.Code != 404 {
		t.Errorf("unknown = %d", rec.Code)
	}
	for _, body := range []string{`{"name":"../x"}`, `{"name":"a","x":1}`, `{"name":""}`, `nope`, ``} {
		if rec := e.send("POST", deletePath, body); rec.Code != 400 {
			t.Errorf("%q = %d, want 400", body, rec.Code)
		}
	}
}

func TestDeleteNeedsEveryGuard(t *testing.T) {
	cases := map[string]func(*http.Request){
		"no edit header":  drop("X-Clalt-Edit"),
		"edit header 0":   set("X-Clalt-Edit", "0"),
		"no origin":       drop("Origin"),
		"foreign origin":  set("Origin", "http://evil.example"),
		"foreign referer": func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Referer", "http://evil.example/x") },
	}
	for name, mod := range cases {
		e := newLayoutEnv(t)
		saveMine(t, e)
		if rec := e.send("POST", deletePath, `{"name":"mine"}`, mod); rec.Code != 403 {
			t.Errorf("%s = %d, want 403", name, rec.Code)
		}
		if _, err := e.store.Load("mine"); err != nil {
			t.Errorf("%s: the layout was deleted", name)
		}
	}
	e := newLayoutEnv(t)
	saveMine(t, e)
	if rec := e.send("POST", deletePath, `{"name":"mine"}`, noCookie); rec.Code != 401 {
		t.Errorf("no cookie = %d", rec.Code)
	}
	if rec := e.send("POST", deletePath, `{"name":"mine"}`, set("Content-Type", "text/plain")); rec.Code != 415 {
		t.Errorf("text/plain = %d", rec.Code)
	}
	big := `{"name":"mine","pad":"` + strings.Repeat("a", layout.MaxBytes) + `"}`
	if rec := e.send("POST", deletePath, big); rec.Code != 413 {
		t.Errorf("oversize = %d", rec.Code)
	}
	for _, m := range []string{"GET", "PUT", "DELETE"} {
		if rec := e.send(m, deletePath, `{"name":"mine"}`); rec.Code != 405 {
			t.Errorf("%s = %d, want 405", m, rec.Code)
		}
	}
	if _, err := e.store.Load("mine"); err != nil {
		t.Error("a refused request deleted the layout")
	}
}

func TestDeletingTheActiveLayoutSwitchesToTheDefaultTemplate(t *testing.T) {
	e := newLayoutEnv(t)
	saveMine(t, e)
	if err := e.store.SetActive("mine"); err != nil {
		t.Fatal(err)
	}
	rec := e.send("POST", deletePath, `{"name":"mine"}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var r deleteReply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Active != layout.DefaultActive {
		t.Errorf("reply active %q %v", r.Active, err)
	}
	if e.store.ActiveName() != layout.DefaultActive {
		t.Errorf("active.json = %q", e.store.ActiveName())
	}
	if rec := e.send("GET", "/api/layout", ""); rec.Code != 200 {
		t.Errorf("the active layout does not load: %d", rec.Code)
	}
}

func TestAFailedDeleteLeavesActiveWhereItWas(t *testing.T) {
	e := newLayoutEnv(t)
	saveMine(t, e)
	if err := e.store.SetActive("mine"); err != nil {
		t.Fatal(err)
	}
	// a directory with that name makes the removal fail, and the layout unloadable
	if err := os.Remove(e.store.Root + "/layouts/mine.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.store.Root+"/layouts/mine.json/x", 0o700); err != nil {
		t.Fatal(err)
	}
	rec := e.send("POST", deletePath, `{"name":"mine"}`)
	if rec.Code < 400 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := e.store.ActiveName()
	if !layout.IsTemplate(got) {
		if _, err := e.store.Load(got); err != nil {
			t.Errorf("active %q does not load: %v", got, err)
		}
	}
}
