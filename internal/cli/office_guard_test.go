package cli

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/office"
)

// guardApp is an App with a temp HOME so officeReviews touches no real dirs.
func guardApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	app, _, _ := newTestApp(t)
	return app
}

func TestOfficeReviewsNeverOverridesAnExplicitAbsoluteClaudeBin(t *testing.T) {
	const want = "/nonexistent/dir/claude"
	t.Setenv("CLALT_CLAUDE_BIN", want)
	svc := guardApp(t).officeReviews(office.Real{}, func(string, ...any) {})
	if svc.ClaudeBin != want {
		t.Fatalf("ClaudeBin = %q, want %q", svc.ClaudeBin, want)
	}
	got, err := svc.R.Resolve(svc.ClaudeBin)
	if err != nil || got != want {
		t.Errorf("Resolve(%q) = %q, %v; want it unchanged", svc.ClaudeBin, got, err)
	}
}

func TestOfficeReviewsWithoutClaudeBinHasABinAndResolvers(t *testing.T) {
	t.Setenv("CLALT_CLAUDE_BIN", "")
	svc := guardApp(t).officeReviews(office.Real{}, func(string, ...any) {})
	if svc.ClaudeBin == "" {
		t.Error("ClaudeBin is empty")
	}
	if svc.R == nil {
		t.Error("svc.R is nil")
	}
	if svc.GH.R == nil {
		t.Error("svc.GH.R is nil")
	}
}
