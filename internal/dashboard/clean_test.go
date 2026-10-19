package dashboard

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode"
)

func TestCleanStripsEscapeSequencesAndBlanksControlCharacters(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "robin", "robin"},
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"hyperlink ended by bell", "\x1b]8;;http://x.example\x07link\x1b]8;;\x07", "link"},
		{"hyperlink ended by string terminator", "\x1b]8;;http://x.example\x1b\\link\x1b]8;;\x1b\\", "link"},
		{"window title", "\x1b]0;evil\x07ok", "ok"},
		{"tab", "a\tb", "a b"},
		{"newline and return", "a\r\nb", "a  b"},
		{"delete", "a\x7fb", "a b"},
		{"c1 control", "a\u0085b", "a b"},
		{"bell", "a\x07b", "a b"},
		{"nul", "a\x00b", "a b"},
		{"empty", "", ""},
		{"spaces stay", "  a  b ", "  a  b "},
		{"text with accents", "caf\u00e9", "caf\u00e9"},
	}
	for _, c := range cases {
		if got := clean(c.in); got != c.want {
			t.Errorf("%s: clean(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestCleanNeverLeavesAControlCharacterWhateverComesIn(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	pieces := []string{"\x1b", "[", "]", "8", ";", "31m", "\x07", "\\", "\t", "\n", "a", " ", "\u009b", "\u0085", "\x7f", "\x00", "http://x", "\xff", "\u00e9"}
	for i := 0; i < 2000; i++ {
		var sb strings.Builder
		for n := rng.Intn(12); n >= 0; n-- {
			sb.WriteString(pieces[rng.Intn(len(pieces))])
		}
		out := clean(sb.String())
		for _, r := range out {
			if unicode.IsControl(r) {
				t.Fatalf("clean(%q) = %q still has the control character %U", sb.String(), out, r)
			}
		}
	}
}

const hostile = "a\tb\n\x1b]8;;http://x.example\x07c\x1b[31md\x7fe"

// fill sets every string below v to the hostile text, leaving ids alone, and
// makes every slice two long so the walk reaches what is inside it
func fill(v reflect.Value, name string) {
	switch v.Kind() {
	case reflect.String:
		if name != "ID" {
			v.SetString(hostile)
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 2, 2))
		for i := 0; i < v.Len(); i++ {
			fill(v.Index(i), name)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); f.CanSet() {
				fill(f, v.Type().Field(i).Name)
			}
		}
	}
}

func strings0(v reflect.Value, name string, visit func(name, s string)) {
	switch v.Kind() {
	case reflect.String:
		visit(name, v.String())
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			strings0(v.Index(i), name, visit)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); f.CanInterface() {
				strings0(f, v.Type().Field(i).Name, visit)
			}
		}
	}
}

// every text field in the snapshot is cleaned, including the ones added later:
// the test fills them by reflection
func TestEveryTextFieldInASnapshotIsCleaned(t *testing.T) {
	var snap Snapshot
	fill(reflect.ValueOf(&snap).Elem(), "")
	if len(snap.Accounts) != 2 || len(snap.Accounts[0].Sessions) != 2 || len(snap.Accounts[0].History.Projects[0].Activity) != 2 {
		t.Fatal("setup: the fill reaches the nested lists")
	}
	got := cleanSnapshot(snap)
	seen := 0
	strings0(reflect.ValueOf(got), "", func(name, s string) {
		seen++
		if name == "ID" {
			if s != hostile && s != "" {
				t.Errorf("an id is left as the source wrote it: %q", s)
			}
			return
		}
		if strings.ContainsFunc(s, unicode.IsControl) {
			t.Errorf("field %s still has a control character: %q", name, s)
		}
	})
	if seen < 30 {
		t.Errorf("the walk should see every text field, saw %d", seen)
	}
	if got.Accounts[0].Label != "a b cd e" {
		t.Errorf("the text is kept, only made safe: %q", got.Accounts[0].Label)
	}
}

func TestCleaningACopyLeavesTheSourceDataAlone(t *testing.T) {
	snap := fiveSnapshot()
	snap.Accounts[0].Label = "a\tb"
	snap.Accounts[0].Sessions[0].Folder = "x\ny"
	_ = cleanSnapshot(snap)
	if snap.Accounts[0].Label != "a\tb" || snap.Accounts[0].Sessions[0].Folder != "x\ny" {
		t.Error("the source keeps its own data, the dashboard cleans a copy")
	}
}

func TestHostileTextNeverReachesTheTerminal(t *testing.T) {
	snap := fiveSnapshot()
	a := &snap.Accounts[0]
	a.Label = "ka\tmal\x1b]8;;http://x.example\x07here\x1b]8;;\x07"
	a.Email = "robin\n@x.example"
	a.Org = "Org\x1b[31m"
	a.LastIn = "~/a\tb"
	a.Sessions[0].Folder = "~/Projects/\x1b]0;pwned\x07work"
	a.Sessions[0].Branch = "feat/\nx"
	a.Problem = "bad\x1b[2Jthing"
	snap.Warnings = []string{"watch\x07out"}
	m := loadedModel(&fakeSource{snap: snap}, &fakeActions{}, testOptions(), 100, 30)
	for _, keys := range [][]string{nil, {"2"}, {"3"}, {"4"}, {"5"}} {
		next, _ := Press(m, keys...)
		raw := Raw(next)
		for _, bad := range []string{"\x1b]", "\x07", "\x1b[2J", "\t", "pwned\x07"} {
			if strings.Contains(raw, bad) {
				t.Fatalf("view %v leaks %q:\n%q", keys, bad, raw)
			}
		}
	}
	if out := Screen(m); !strings.Contains(out, "ka mal") || !strings.Contains(out, "watch out") {
		t.Errorf("the text is still there, with spaces where the controls were:\n%s", out)
	}
}

func TestAWarningOfOnlyControlCharactersIsNotShown(t *testing.T) {
	snap := fiveSnapshot()
	snap.Warnings = []string{"\x1b[31m", "\t\n"}
	m := loadedModel(&fakeSource{snap: snap}, &fakeActions{}, testOptions(), 100, 30)
	if m.warnRows() != 0 || len(m.snap.Warnings) != 0 {
		t.Errorf("nothing readable is left, so nothing is shown: %q", m.snap.Warnings)
	}
}

func TestACleanedLabelStillFindsItsAccountByID(t *testing.T) {
	snap := fiveSnapshot()
	snap.Accounts[1].Label = "pra\ttik"
	act := &fakeActions{}
	m := loadedModel(&fakeSource{snap: snap}, act, testOptions(), 100, 30)
	pressSettle(m, "j", "enter")
	if got := act.log(); len(got) != 1 || got[0] != "run a2" {
		t.Errorf("actions address the account by id: %v", got)
	}
}
