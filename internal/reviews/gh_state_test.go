package reviews

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPRStateMapsAnswers(t *testing.T) {
	cases := map[string]string{
		`{"state":"OPEN"}`:   "open",
		`{"state":"MERGED"}`: "merged",
		`{"state":"CLOSED"}`: "closed",
	}
	for out, want := range cases {
		fx := (&fakeExec{}).on("pr view", out, nil)
		got, err := GH{X: fx}.PRState(context.Background(), "owner/name", 42)
		if err != nil || got != want {
			t.Errorf("%s -> %q, %v; want %q", out, got, err, want)
		}
		lines := fx.lines()
		if len(lines) != 1 || lines[0] != "gh pr view 42 --repo owner/name --json state" {
			t.Errorf("argv = %v", lines)
		}
	}
}

func TestPRStateRejectsOddAnswers(t *testing.T) {
	for _, out := range []string{`{"state":"DRAFT"}`, `{"state":""}`, `{}`, `not json`, ``} {
		fx := (&fakeExec{}).on("pr view", out, nil)
		got, err := GH{X: fx}.PRState(context.Background(), "owner/name", 42)
		if err == nil || got != "" {
			t.Errorf("%q -> %q, %v; want empty and an error", out, got, err)
		}
	}
}

func TestPRStateExecError(t *testing.T) {
	fx := (&fakeExec{}).on("pr view", "", errors.New("boom"))
	got, err := GH{X: fx}.PRState(context.Background(), "owner/name", 42)
	if err == nil || got != "" {
		t.Fatalf("got %q, %v; want empty and an error", got, err)
	}
}

func TestAllowedGHAcceptsStateLookup(t *testing.T) {
	if err := allowedGH([]string{"pr", "view", "42", "--repo", "owner/name", "--json", "state"}); err != nil {
		t.Fatalf("refused: %v", err)
	}
}

// pins the allowlist as it is today so the state lookup cannot widen it.
func TestGHAllowedNotWidened(t *testing.T) {
	want := map[string]ghRule{
		"search prs":  rule([]string{"--repo", "--json", "--limit"}, "--review-requested=@me", "--state=open"),
		"pr view":     rule([]string{"--repo", "--json"}),
		"pr diff":     rule([]string{"--repo"}),
		"pr checkout": rule([]string{"--repo"}, "--detach"),
		"repo list":   rule([]string{"--json", "--limit"}, "--no-archived", "--source"),
		"repo clone":  rule(nil),
	}
	if !reflect.DeepEqual(ghAllowed, want) {
		t.Fatalf("ghAllowed changed:\n got %+v\nwant %+v", ghAllowed, want)
	}
}

func TestPRViewRefusesExtraFlags(t *testing.T) {
	base := []string{"pr", "view", "42", "--repo", "owner/name", "--json", "state"}
	extras := [][]string{
		{"--state", "open"},
		{"-X", "POST"},
		{"--body", "x"},
		{"--frobnicate"},
	}
	for _, e := range extras {
		argv := append(append([]string{}, base...), e...)
		if err := allowedGH(argv); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%v allowed: %v", argv, err)
		}
	}
}
