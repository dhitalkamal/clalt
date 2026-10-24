package office

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
	"github.com/dhitalkamal/clalt/internal/office/layout/garden"
)

// v2 returns a small generated garden layout under a name.
func v2(t *testing.T, name string) layout.Layout {
	t.Helper()
	l, err := garden.Generate("rows", garden.Input{
		Accounts:       []garden.Account{{ID: "work", Label: "WORK"}},
		CabinDesks:     map[string]int{"work": 2},
		WorkspaceDesks: 8, ResearchDesks: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	l.Name, l.Template = name, ""
	return l
}

func TestHallEditWhileATemplateIsActiveSavesAsHallCustom(t *testing.T) {
	e := newLayoutEnv(t, true)
	if err := e.store.SetActive("rows"); err != nil {
		t.Fatal(err)
	}
	l := layout.Minimal()
	l.Title = "Hall edit"
	rec := e.send("PUT", "/api/layout", string(mustEncode(t, l)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := e.store.ActiveName(); got != "hall-custom" {
		t.Errorf("active = %q, want hall-custom", got)
	}
	if _, err := e.store.Load("rows-custom"); err == nil {
		t.Error("a hall edit was saved as rows-custom")
	}
}

func TestV2EditWhileATemplateIsActiveSavesAsTemplateCustom(t *testing.T) {
	e := newLayoutEnv(t, true)
	if err := e.store.SetActive("rows"); err != nil {
		t.Fatal(err)
	}
	rec := e.send("PUT", "/api/layout", string(mustEncode(t, v2(t, "x"))))
	if rec.Code != 200 || e.store.ActiveName() != "rows-custom" {
		t.Errorf("%d active %q %s", rec.Code, e.store.ActiveName(), rec.Body)
	}
}

func TestHallEditNeverOverwritesAStoredV2Layout(t *testing.T) {
	e := newLayoutEnv(t, true)
	if err := e.store.Save(v2(t, "foo")); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActive("foo"); err != nil {
		t.Fatal(err)
	}
	rec := e.send("PUT", "/api/layout", string(mustEncode(t, layout.Minimal())))
	if rec.Code != 409 {
		t.Fatalf("code %d, want 409: %s", rec.Code, rec.Body)
	}
	if got, err := e.store.Load("foo"); err != nil || got.V != layout.Version {
		t.Errorf("foo changed: %v v%d", err, got.V)
	}
}

func TestV2EditNeverOverwritesAStoredHallLayout(t *testing.T) {
	e := newLayoutEnv(t, true)
	mine := layout.Minimal()
	mine.Name = "mine"
	if err := e.store.Save(mine); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActive("mine"); err != nil {
		t.Fatal(err)
	}
	if rec := e.send("PUT", "/api/layout", string(mustEncode(t, v2(t, "mine")))); rec.Code != 409 {
		t.Fatalf("code %d, want 409: %s", rec.Code, rec.Body)
	}
	if got, _ := e.store.Load("mine"); got.V != layout.V1 {
		t.Errorf("mine is v%d", got.V)
	}
}
