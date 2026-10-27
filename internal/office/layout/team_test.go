package layout

import (
	"fmt"
	"strings"
	"testing"
)

// withTeam is valid2 plus one team pavilion of two by two desks.
func withTeam(members ...string) Layout {
	l := valid2()
	l.Pavilions = append(l.Pavilions, Pavilion{ID: "team-1", Type: "team", X: -10, Z: 13, Cols: 2, Rows: 2, Members: members})
	return l
}

func TestTeamFootprintLikeWorkspace(t *testing.T) {
	w, d, ok := Footprint("team", 2, 2)
	ww, wd, _ := Footprint("workspace", 2, 2)
	if !ok || w != ww || d != wd {
		t.Errorf("team footprint %v x %v ok=%v, workspace %v x %v", w, d, ok, ww, wd)
	}
	if !HasDesks("team") || !IsPavilionType("team") {
		t.Error("team is a desk pavilion type")
	}
}

func TestTeamPavilionValidates(t *testing.T) {
	for _, l := range []Layout{withTeam(), withTeam("explore"), withTeam("general-purpose", "explore", "plan", "pr-review-toolkit:code-reviewer")} {
		if err := Validate(l, nil); err != nil {
			t.Errorf("%v: %v", l.Pavilions[3].Members, err)
		}
	}
	// exactly as many members as desks
	eight := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8"}
	if err := Validate(withTeam(eight...), nil); err != nil {
		t.Errorf("8 members on 8 desks: %v", err)
	}
}

func TestTeamMemberRules(t *testing.T) {
	cases := []struct {
		name string
		mod  func(l *Layout)
		want string
	}{
		{"upper case", func(l *Layout) { l.Pavilions[3].Members = []string{"Explore"} }, "member"},
		{"space", func(l *Layout) { l.Pavilions[3].Members = []string{"a b"} }, "member"},
		{"empty id", func(l *Layout) { l.Pavilions[3].Members = []string{""} }, "member"},
		{"long id", func(l *Layout) { l.Pavilions[3].Members = []string{strings.Repeat("a", 65)} }, "member"},
		{"slash", func(l *Layout) { l.Pavilions[3].Members = []string{"a/b"} }, "member"},
		{"duplicate", func(l *Layout) { l.Pavilions[3].Members = []string{"a", "a"} }, "duplicate"},
		{"more than desks", func(l *Layout) {
			for i := 0; i < 9; i++ {
				l.Pavilions[3].Members = append(l.Pavilions[3].Members, fmt.Sprintf("m%d", i))
			}
		}, "desks"},
		{"members on workspace", func(l *Layout) { l.Pavilions[2].Members = []string{"a"} }, "members"},
		{"members on lobby", func(l *Layout) { l.Pavilions[0].Members = []string{"a"} }, "members"},
		{"team without cols", func(l *Layout) { l.Pavilions[3].Cols = 0 }, "cols"},
	}
	for _, c := range cases {
		l := withTeam()
		c.mod(&l)
		err := Validate(l, nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

func TestTeamMembersRoundTrip(t *testing.T) {
	l := withTeam("explore", "plan")
	raw, err := Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"members"`) {
		t.Error("members missing from the encoding")
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(got, nil); err != nil {
		t.Fatal(err)
	}
	if m := got.Pavilions[3].Members; len(m) != 2 || m[0] != "explore" || m[1] != "plan" {
		t.Errorf("members after decode: %v", m)
	}
	// other pavilions never gain the key
	plain, _ := Encode(valid2())
	if strings.Contains(string(plain), "members") {
		t.Error("a layout without a team holds the members key")
	}
}
