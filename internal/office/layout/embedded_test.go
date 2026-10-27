package layout

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// every template that ships must pass the validator against the shipped
// catalog. both come from the page builder, so this is the check that the two
// sides agree. it does nothing while neither file is there yet.
func TestEmbeddedTemplatesValidateAgainstTheEmbeddedCatalog(t *testing.T) {
	cat, err := EmbeddedCatalog()
	if err != nil {
		t.Skipf("no embedded catalog yet: %v", err)
	}
	ents, _ := fs.ReadDir(embeddedFS, "data/templates")
	s := NewStore(t.TempDir(), nil)
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if _, err := s.Load(name); err != nil {
			t.Errorf("template %s: %v", name, err)
		}
	}
	if _, err := s.Load(DefaultName); err != nil {
		t.Errorf("default does not load: %v (catalog has %d items)", err, len(cat))
	}
}

// the page keeps copies of the catalog and templates under web/layout, which
// must not drift from the ones validated here.
func TestWebCopiesMatchTheEmbeddedFiles(t *testing.T) {
	copies, _ := filepath.Glob(filepath.Join("..", "web", "layout", "*.json"))
	if len(copies) == 0 {
		t.Skip("the page has no layout copies yet")
	}
	for _, c := range copies {
		base := filepath.Base(c)
		src := "data/" + base
		if base != "catalog.json" {
			src = "data/templates/" + base
		}
		want, err := fs.ReadFile(embeddedFS, src)
		if err != nil {
			t.Errorf("%s has no counterpart: %v", c, err)
			continue
		}
		got, _ := os.ReadFile(c)
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s", c, src)
		}
	}
}
