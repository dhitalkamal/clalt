package officed

import (
	"strings"
	"testing"
)

const ghWarn = "warning: gh not found on PATH or in /opt/homebrew/bin, /usr/local/bin, /home/x/.local/bin, /home/x/go/bin; reviews will fail"

var testDirs = []string{"/opt/homebrew/bin", "/usr/local/bin", "/home/x/.local/bin", "/home/x/go/bin"}

// warnLines picks the warning lines out of the status output
func warnLines(out string) []string {
	var got []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "warning:") {
			got = append(got, l)
		}
	}
	return got
}

func TestStatusWarnsAboutMissingGH(t *testing.T) {
	w := newWorld(t)
	w.m.Missing = func() []MissingTool { return []MissingTool{{Name: "gh", Dirs: testDirs}} }
	if err := w.m.Status(false); err != nil {
		t.Fatal(err)
	}
	got := warnLines(w.out.String())
	if len(got) != 1 || got[0] != ghWarn {
		t.Errorf("warnings %q, want [%q]", got, ghWarn)
	}
}

func TestStatusWarnsInGivenOrderForBothTools(t *testing.T) {
	w := newWorld(t)
	w.m.Missing = func() []MissingTool {
		return []MissingTool{{Name: "gh", Dirs: testDirs}, {Name: "claude", Dirs: testDirs}}
	}
	if err := w.m.Status(false); err != nil {
		t.Fatal(err)
	}
	got := warnLines(w.out.String())
	want := strings.Replace(ghWarn, "warning: gh", "warning: claude", 1)
	if len(got) != 2 || got[0] != ghWarn || got[1] != want {
		t.Errorf("warnings %q", got)
	}
}

func TestStatusNoWarningWhenNothingMissing(t *testing.T) {
	w := newWorld(t)
	w.m.Missing = func() []MissingTool { return nil }
	if err := w.m.Status(false); err != nil {
		t.Fatal(err)
	}
	if got := warnLines(w.out.String()); len(got) != 0 {
		t.Errorf("warnings %q", got)
	}
}

func TestStatusNoWarningWhenMissingIsNil(t *testing.T) {
	w := newWorld(t)
	if err := w.m.Status(false); err != nil {
		t.Fatal(err)
	}
	if got := warnLines(w.out.String()); len(got) != 0 {
		t.Errorf("warnings %q", got)
	}
}

func TestStatusWarnsWhenOfficeIsOnToo(t *testing.T) {
	w := newWorld(t)
	w.seed(321, true)
	w.m.Missing = func() []MissingTool { return []MissingTool{{Name: "gh", Dirs: testDirs}} }
	if err := w.m.Status(false); err != nil {
		t.Fatal(err)
	}
	got := warnLines(w.out.String())
	if len(got) != 1 || got[0] != ghWarn {
		t.Errorf("warnings %q", got)
	}
}
