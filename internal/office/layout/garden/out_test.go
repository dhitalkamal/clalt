package garden

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// TestWriteSamples writes <template>-3.json (WORK, PERSONAL, CLIENT) and
// <template>-8.json into GARDEN_OUT_DIR for the renderer to load. it does
// nothing when the variable is unset.
func TestWriteSamples(t *testing.T) {
	dir := os.Getenv("GARDEN_OUT_DIR")
	if dir == "" {
		t.Skip("GARDEN_OUT_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	eight := Input{WorkspaceDesks: 8, ResearchDesks: 4, CabinDesks: map[string]int{}}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("acct%d", i+1)
		eight.Accounts = append(eight.Accounts, Account{ID: id, Label: fmt.Sprintf("ACCOUNT %d", i+1)})
		eight.CabinDesks[id] = 2
	}
	for _, tpl := range templates {
		for n, in := range map[int]Input{3: three(), 8: eight} {
			l, err := Generate(tpl, in)
			if err != nil {
				t.Fatal(err)
			}
			b, err := layout.Encode(l)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%d.json", tpl, n)), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
