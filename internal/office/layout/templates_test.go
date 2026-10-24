package layout

import (
	"errors"
	"testing"
)

func TestTemplateList(t *testing.T) {
	var ids []string
	for _, tp := range Templates {
		if tp.Title == "" || !ValidName(tp.ID) {
			t.Errorf("bad template %+v", tp)
		}
		ids = append(ids, tp.ID)
	}
	want := []string{"rows", "village", "river", "street", "hall"}
	if len(ids) != len(want) {
		t.Fatalf("templates = %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] || !IsTemplate(want[i]) {
			t.Errorf("template %d = %q, want %q", i, ids[i], want[i])
		}
	}
	if IsTemplate("default") || IsTemplate("mine") || IsTemplate("") {
		t.Error("a non template counts as one")
	}
	if DefaultActive != "rows" {
		t.Errorf("default active = %q", DefaultActive)
	}
}

func TestActiveNameDefaultsToRows(t *testing.T) {
	s, _ := newStore(t)
	if s.ActiveName() != "rows" {
		t.Errorf("active = %q", s.ActiveName())
	}
}

func TestSetActiveTakesTemplateNames(t *testing.T) {
	s, _ := newStore(t)
	for _, name := range []string{"village", "hall"} {
		if err := s.SetActive(name); err != nil || s.ActiveName() != name {
			t.Errorf("SetActive(%s) = %v, active %q", name, err, s.ActiveName())
		}
	}
	// a template has no layout file, so Active falls back to the hall
	if s.Active().Name != "default" {
		t.Errorf("active layout of a template name = %q", s.Active().Name)
	}
}

func TestTemplateNamesAreNotUserLayouts(t *testing.T) {
	s, _ := newStore(t)
	for _, name := range []string{"rows", "village", "river", "street", "hall"} {
		l := valid()
		l.Name = name
		if err := s.Save(l); !errors.Is(err, ErrBuiltin) {
			t.Errorf("save %s = %v", name, err)
		}
		if err := s.Delete(name); !errors.Is(err, ErrBuiltin) && !errors.Is(err, ErrNotFound) {
			t.Errorf("delete %s = %v", name, err)
		}
	}
}

func TestSaveStoresCustomV2ButNeverATemplate(t *testing.T) {
	s, _ := newStore(t)
	l := valid2()
	if err := s.Save(l); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("garden")
	if err != nil || got.V != Version || len(got.Pavilions) != 3 {
		t.Fatalf("load = %+v, %v", got, err)
	}
	l.Name, l.Template = "generated", "river"
	if err := s.Save(l); err == nil {
		t.Error("saved a layout that names a template")
	}
	if _, err := s.Load("generated"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a refused save left a file: %v", err)
	}
}

func TestLoadRefusesAStoredTemplateLayout(t *testing.T) {
	s, _ := newStore(t)
	l := valid2()
	l.Name, l.Template = "sneaky", "rows"
	raw, err := Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeUser(s, "sneaky", raw); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("sneaky"); err == nil {
		t.Error("loaded a stored layout with a template")
	}
}
