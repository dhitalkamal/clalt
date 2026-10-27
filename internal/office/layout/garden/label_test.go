package garden

import (
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// hostile labels and ids must still give a layout the server accepts.
func TestHostileLabelsPassValidate(t *testing.T) {
	cases := []Account{
		{"a1", "two  spaces"},
		{"a2", "visit www.evil.com"},
		{"www.x", ""},
		{"a4", "zero\u200bwidth"},
		{"a5", strings.Repeat("x", 39) + " y"},
		{"a6", "http://x"},
		{"a7", "tab\there"},
		{"www.b", "www.b"},
	}
	in := Input{Accounts: cases, CabinDesks: map[string]int{}, WorkspaceDesks: 8, ResearchDesks: 4}
	for _, tpl := range templates {
		l, err := Generate(tpl, in)
		if err != nil {
			t.Fatalf("%s: %v", tpl, err)
		}
		if err := layout.Validate(l, layout.Catalog{}); err != nil {
			t.Errorf("%s: %v", tpl, err)
		}
	}
}
