package reviews

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

const cutMark = "[diff cut here]"

// diffPart is what follows the "The diff:" label in a prompt, "" when the
// prompt has none (the cross-file pass).
func diffPart(stdin string) string {
	i := strings.LastIndex(stdin, "The diff:\n")
	if i < 0 {
		return ""
	}
	return stdin[i+len("The diff:\n"):]
}

// bigRig is a rig that reviews only #7 with the given diff and the default
// budget. every claude run gets an empty review.
func bigRig(t *testing.T, diff string) *rig {
	t.Helper()
	r := newRig(t)
	onlySmallPR(r)
	r.fx.rules = append([]fakeRule{{has: "pr diff 7", out: diff}}, r.fx.rules...)
	r.fx.answer = func(c call) (string, bool) {
		if c.Name != "claude" {
			return "", false
		}
		return envelope("s", "ok", ""), true
	}
	return r
}

func TestBuildPromptKeepsAWholeDiffOverTheOldCap(t *testing.T) {
	var b strings.Builder
	b.WriteString("diff --git a/a.go b/a.go\n")
	for i := 0; b.Len() < 600*1024; i++ {
		fmt.Fprintf(&b, "+line_%06d_of_the_huge_diff\n", i)
	}
	diff := b.String()
	got := buildPrompt(PR{Number: 7, Repo: "o/r"}, PRInfo{Title: "t"}, diff)
	if strings.Contains(got, cutMark) {
		t.Errorf("prompt has %q", cutMark)
	}
	if !strings.HasSuffix(got, "The diff:\n"+diff) {
		t.Errorf("prompt does not end with the whole %d byte diff", len(diff))
	}
}

func TestTickReviewsEveryLineOfA450KBDiffInChunks(t *testing.T) {
	var b strings.Builder
	var unique []string
	for f := 0; f < 30; f++ {
		fmt.Fprintf(&b, "diff --git a/f%02d.go b/f%02d.go\n", f, f)
		start := b.Len()
		for l := 0; b.Len()-start < 15*1024; l++ {
			line := fmt.Sprintf("+file_%02d_line_%04d_padding_padding_padding", f, l)
			unique = append(unique, line)
			b.WriteString(line + "\n")
		}
	}
	diff := b.String()
	if len(diff) < 440*1024 {
		t.Fatalf("test diff is only %d bytes", len(diff))
	}
	r := bigRig(t, diff)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := r.claudeCalls()
	if len(calls) < 3 {
		t.Fatalf("claude ran %d times, want several chunks and a cross-file pass", len(calls))
	}
	seen := map[string]int{}
	for i, c := range calls {
		if strings.Contains(c.Stdin, cutMark) {
			t.Errorf("run %d stdin has %q", i, cutMark)
		}
		part := diffPart(c.Stdin)
		if i == len(calls)-1 {
			if !strings.Contains(c.Stdin, "cross-file") || part != "" || strings.Contains(c.Stdin, "+file_") {
				t.Errorf("last run is not a diffless cross-file pass: %q", c.Stdin)
			}
			continue
		}
		for _, ln := range strings.Split(part, "\n") {
			if strings.HasPrefix(ln, "+file_") {
				seen[ln]++
			}
		}
	}
	for _, ln := range unique {
		if seen[ln] != 1 {
			t.Fatalf("line %q is in %d chunk prompts, want 1", ln, seen[ln])
		}
	}
}

func TestTickKeepsAnOverLongLineWhole(t *testing.T) {
	long := "+" + strings.Repeat("x", 450*1024)
	diff := "diff --git a/big.txt b/big.txt\n" + long + "\n"
	r := bigRig(t, diff)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	whole := 0
	for i, c := range r.claudeCalls() {
		if strings.Contains(c.Stdin, cutMark) {
			t.Errorf("run %d stdin has %q", i, cutMark)
		}
		if strings.Contains(diffPart(c.Stdin), long+"\n") {
			whole++
		}
	}
	if whole != 1 {
		t.Errorf("the long line is whole in %d prompts, want 1", whole)
	}
}
