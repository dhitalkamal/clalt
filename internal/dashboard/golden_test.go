package dashboard_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// regenerate with: go test ./internal/dashboard -run 'TestGoldenFrames|TestFramesDoc' -update
var update = flag.Bool("update", false, "rewrite the golden frames in testdata")

type frameSpec struct {
	name string
	w, h int
	keys []string
}

func frameSpecs() []frameSpec {
	history := func(extra ...string) []string { return append([]string{"4"}, extra...) }
	// rename opens the prompt, clears robin and types a name
	rename := func(name string) []string {
		keys := []string{"r", "backspace", "backspace", "backspace", "backspace", "backspace"}
		for _, r := range name {
			keys = append(keys, string(r))
		}
		return keys
	}
	return []frameSpec{
		{"robin-overview-100x30", 100, 30, nil},
		{"robin-sessions-100x30", 100, 30, []string{"2"}},
		{"robin-limits-100x30", 100, 30, []string{"3"}},
		{"robin-history-days-100x30", 100, 30, history()},
		{"robin-history-projects-100x30", 100, 30, history("tab", "l")},
		{"robin-history-models-100x30", 100, 30, history("tab", "l", "l")},
		{"robin-drill-clalt-100x30", 100, 30, history("tab", "l", "enter")},
		{"robin-profile-100x30", 100, 30, []string{"5"}},
		{"casey-overview-100x30", 100, 30, []string{"j", "j"}},
		{"harriet-overview-100x30", 100, 30, []string{"j", "j", "j", "j"}},
		{"robin-overview-80x24", 80, 24, nil},
		{"one-pane-accounts-64x20", 64, 20, nil},
		{"one-pane-detail-64x20", 64, 20, []string{"tab"}},
		{"too-small-50x14", 50, 14, nil},
		{"help-100x30", 100, 30, []string{"?"}},
		{"filter-sh-100x30", 100, 30, []string{"/", "s", "h"}},
		{"flash-enter-100x30", 100, 30, []string{"enter"}},
		{"stop-question-100x30", 100, 30, []string{"2", "x"}},
		{"budget-entry-100x30", 100, 30, []string{"e"}},
		{"sign-out-confirm-100x30", 100, 30, []string{"d"}},
		{"delete-box-100x30", 100, 30, []string{"D"}},
		{"rename-prompt-100x30", 100, 30, []string{"r"}},
		{"rename-error-100x30", 100, 30, append(rename("morgan"), "enter")},
	}
}

// frameTitles name the frames in the plan's design reference
var frameTitles = map[string]string{
	"robin-overview-100x30":         "robin, overview, 100x30",
	"robin-sessions-100x30":         "robin, sessions, 100x30",
	"robin-limits-100x30":           "robin, limits, 100x30",
	"robin-history-days-100x30":     "robin, history days, 100x30",
	"robin-history-projects-100x30": "robin, history projects, 100x30",
	"robin-history-models-100x30":   "robin, history models, 100x30",
	"robin-drill-clalt-100x30":      "robin, history projects, drilled into clalt, 100x30",
	"robin-profile-100x30":          "robin, profile, 100x30",
	"casey-overview-100x30":         "casey, overview, 100x30",
	"harriet-overview-100x30":       "harriet, overview, 100x30",
	"robin-overview-80x24":          "robin, overview, 80x24",
	"one-pane-accounts-64x20":       "one pane, accounts focused, 64x20",
	"one-pane-detail-64x20":         "one pane, detail focused, 64x20",
	"too-small-50x14":               "too small, 50x14",
	"help-100x30":                   "help overlay, 100x30",
	"filter-sh-100x30":              "filter sh typed, 100x30",
	"flash-enter-100x30":            "flash after enter, 100x30",
	"stop-question-100x30":          "stop question, 100x30",
	"budget-entry-100x30":           "budget entry, 100x30",
	"sign-out-confirm-100x30":       "sign out question, 100x30",
	"delete-box-100x30":             "delete box, 100x30",
	"rename-prompt-100x30":          "rename prompt, 100x30",
	"rename-error-100x30":           "rename prompt after a mistake, 100x30",
}

const framesDocPath = "../../docs/plans/dashboard-v2-frames.txt"

// framesDoc is the design reference in the plan: its own first line, then every
// golden frame in order under its title
func framesDoc(t *testing.T) string {
	t.Helper()
	old, err := os.ReadFile(framesDocPath)
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(string(old), "\n")
	var sb strings.Builder
	sb.WriteString(header + "\n")
	for _, f := range frameSpecs() {
		raw, err := os.ReadFile(filepath.Join("testdata", f.name+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		title, ok := frameTitles[f.name]
		if !ok {
			t.Fatalf("frame %s has no title for the plan", f.name)
		}
		sb.WriteString("\nframe: " + title + "\n\n" + string(raw))
	}
	return sb.String()
}

// renderFrame is one frame in ascii mode without color, trailing spaces cut
func renderFrame(f frameSpec) string {
	m := sized(config{ascii: true, noColor: true}, f.w, f.h)
	m, cmd := press(m, f.keys...)
	m = settle(m, cmd)
	var sb strings.Builder
	for _, l := range strings.Split(screen(m), "\n") {
		sb.WriteString(strings.TrimRight(l, " ") + "\n")
	}
	return sb.String()
}

func TestGoldenFrames(t *testing.T) {
	for _, f := range frameSpecs() {
		t.Run(f.name, func(t *testing.T) {
			got := renderFrame(f)
			if !dashboard.IsASCII(got) || strings.Contains(got, "\x1b") {
				t.Fatal("golden frames are plain ascii with no escapes")
			}
			path := filepath.Join("testdata", f.name+".txt")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden file, create it with -update: %v", err)
			}
			if got != string(want) {
				t.Errorf("%s differs from its golden frame.\n--- got\n%s--- want\n%s", f.name, got, want)
			}
		})
	}
}

func TestEveryGoldenFileHasASpec(t *testing.T) {
	known := map[string]bool{}
	for _, f := range frameSpecs() {
		known[f.name+".txt"] = true
	}
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !known[e.Name()] {
			t.Errorf("testdata/%s has no frame spec, remove it or add the spec", e.Name())
		}
	}
}

// the design reference in docs/plans is the golden frames, so the plan cannot
// drift from what the dashboard draws. -update rewrites it with the frames.
func TestFramesDocMatchesTheGoldens(t *testing.T) {
	want := framesDoc(t)
	if *update {
		if err := os.WriteFile(framesDocPath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(framesDocPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Error("docs/plans/dashboard-v2-frames.txt differs from the golden frames, rewrite it with go test ./internal/dashboard -run 'TestGoldenFrames|TestFramesDoc' -update")
	}
}
