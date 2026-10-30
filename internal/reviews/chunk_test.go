package reviews

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fileHeader is the header block of one file section (no hunks).
func fileHeader(name string) string {
	return "diff --git a/" + name + " b/" + name + "\n" +
		"index 1111111..2222222 100644 " + name + "\n" + // name makes the line unique per file
		"--- a/" + name + "\n" +
		"+++ b/" + name + "\n"
}

// hunkText builds one hunk: the @@ line then each line plus a newline.
func hunkText(at string, lines ...string) string {
	return "@@ " + at + " @@ ctx\n" + strings.Join(lines, "\n") + "\n"
}

// numbered returns n unique content lines like "+tag-03" (no terminator).
func numbered(tag string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("+%s-%02d", tag, i)
	}
	return out
}

// keepLines splits on "\n" keeping each terminator.
func keepLines(s string) []string {
	parts := strings.SplitAfter(s, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// joinChunks concatenates the Text of all chunks.
func joinChunks(cs []chunk) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Text)
	}
	return b.String()
}

// countIn counts how many times a whole line (with its terminator)
// occurs, summed over all chunks.
func countIn(cs []chunk, line string) int {
	n := 0
	for _, c := range cs {
		for _, l := range keepLines(c.Text) {
			if l == line {
				n++
			}
		}
	}
	return n
}

func TestSmallDiffStaysInOneChunkAndIsReproducedExactly(t *testing.T) {
	diff := fileHeader("a.go") + hunkText("-1,2 +1,2", " keep", "-old", "+new")
	got := splitDiff(diff, 10000)
	if len(got) != 1 {
		t.Fatalf("got %d chunks, want 1", len(got))
	}
	if got[0].Text != diff {
		t.Errorf("text = %q, want %q", got[0].Text, diff)
	}
	if !reflect.DeepEqual(got[0].Files, []string{"a.go"}) {
		t.Errorf("files = %v, want [a.go]", got[0].Files)
	}
}

func TestSeveralFilesArePackedGreedilyUnderBudget(t *testing.T) {
	sec := func(n string) string { return fileHeader(n) + hunkText("-1 +1", numbered(n, 3)...) }
	a, b, c := sec("a.go"), sec("b.go"), sec("c.go")
	budget := len(a) + len(b) + 5 // two fit, three do not
	got := splitDiff(a+b+c, budget)
	if len(got) != 2 {
		t.Fatalf("got %d chunks, want 2: %+v", len(got), got)
	}
	if got[0].Text != a+b || got[1].Text != c {
		t.Errorf("texts = %q and %q", got[0].Text, got[1].Text)
	}
	if !reflect.DeepEqual(got[0].Files, []string{"a.go", "b.go"}) ||
		!reflect.DeepEqual(got[1].Files, []string{"c.go"}) {
		t.Errorf("files = %v and %v", got[0].Files, got[1].Files)
	}
	if joinChunks(got) != a+b+c {
		t.Error("concatenated text differs from the input")
	}
}

func TestBigFileIsSplitAtHunkBoundariesWithHeaderRepeated(t *testing.T) {
	hdr := fileHeader("big.go")
	var hunks []string
	for i := 0; i < 4; i++ {
		hunks = append(hunks, hunkText(fmt.Sprintf("-%d,3 +%d,3", i*10, i*10), numbered(fmt.Sprintf("h%d", i), 3)...))
	}
	diff := hdr + strings.Join(hunks, "")
	budget := len(hdr) + len(hunks[0]) + len(hunks[1]) + 3 // two hunks per piece
	got := splitDiff(diff, budget)
	if len(got) < 2 {
		t.Fatalf("got %d chunks, want a split", len(got))
	}
	for i, c := range got {
		if !strings.HasPrefix(c.Text, "diff --git ") || !strings.HasPrefix(c.Text, hdr) {
			t.Errorf("piece %d does not start with the file header: %q", i, c.Text)
		}
		if !strings.Contains(c.Text, "--- ") || !strings.Contains(c.Text, "+++ ") {
			t.Errorf("piece %d lacks --- or +++ lines", i)
		}
		if len(c.Text) > budget {
			t.Errorf("piece %d is %d bytes, over budget %d", i, len(c.Text), budget)
		}
		if !reflect.DeepEqual(c.Files, []string{"big.go"}) {
			t.Errorf("piece %d files = %v", i, c.Files)
		}
		if !strings.HasPrefix(strings.TrimPrefix(c.Text, hdr), "@@ ") {
			t.Errorf("piece %d has no hunk right after the header", i)
		}
	}
	// whole hunks only: each hunk text appears in exactly one piece
	for i, h := range hunks {
		n := 0
		for _, c := range got {
			n += strings.Count(c.Text, h)
		}
		if n != 1 {
			t.Errorf("hunk %d appears %d times, want 1", i, n)
		}
	}
}

func TestOversizedHunkIsSplitAtLinesWithAtLineRepeated(t *testing.T) {
	hdr := fileHeader("h.go")
	lines := numbered("row", 12)
	hunk := hunkText("-1,12 +1,12", lines...)
	atLine := keepLines(hunk)[0]
	budget := len(hdr) + len(atLine) + 3*len("+row-00\n") // three lines per piece
	got := splitDiff(hdr+hunk, budget)
	if len(got) < 4 {
		t.Fatalf("got %d chunks, want at least 4", len(got))
	}
	for i, c := range got {
		if !strings.HasPrefix(c.Text, hdr+atLine) {
			t.Errorf("piece %d is not header then @@ line: %q", i, c.Text)
		}
		if len(c.Text) > budget {
			t.Errorf("piece %d is %d bytes, over budget %d", i, len(c.Text), budget)
		}
	}
	for _, l := range lines {
		if n := countIn(got, l+"\n"); n != 1 {
			t.Errorf("line %q appears %d times, want 1", l, n)
		}
	}
	if n := countIn(got, atLine); n != len(got) {
		t.Errorf("@@ line appears %d times, want once per piece (%d)", n, len(got))
	}
}

func TestEmptyDiffGivesOneEmptyChunk(t *testing.T) {
	got := splitDiff("", 100)
	if len(got) != 1 {
		t.Fatalf("got %d chunks, want 1", len(got))
	}
	if got[0].Text != "" || len(got[0].Files) != 0 {
		t.Errorf("chunk = %+v, want empty text and no files", got[0])
	}
}

func TestBinaryAndRenameSectionsWithoutHunksAreKept(t *testing.T) {
	bin := "diff --git a/img.png b/img.png\nindex 1..2 100644\nBinary files a/img.png and b/img.png differ\n"
	ren := "diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n"
	mode := "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n"
	diff := bin + ren + mode
	got := splitDiff(diff, 10000)
	if len(got) != 1 || got[0].Text != diff {
		t.Fatalf("got %+v, want one chunk with the exact input", got)
	}
	if want := []string{"img.png", "new.go", "run.sh"}; !reflect.DeepEqual(got[0].Files, want) {
		t.Errorf("files = %v, want %v", got[0].Files, want)
	}
	// with a tight budget each header-only section is packed like any file
	tight := splitDiff(diff, len(ren)+1)
	if joinChunks(tight) != diff {
		t.Error("tight budget lost or changed text")
	}
	if len(tight) < 2 {
		t.Errorf("got %d chunks, want the sections spread over several", len(tight))
	}
}

func TestCRLFLinesSurviveByteForByte(t *testing.T) {
	hdr := fileHeader("w.txt")
	var lines []string
	for i := 0; i < 8; i++ {
		lines = append(lines, fmt.Sprintf("+crlf-%02d\r\n", i))
	}
	atLine := "@@ -1,8 +1,8 @@\r\n"
	diff := hdr + atLine + strings.Join(lines, "")
	// fits: reproduced exactly
	if got := splitDiff(diff, 10000); len(got) != 1 || got[0].Text != diff {
		t.Fatalf("unsplit crlf diff changed: %+v", got)
	}
	// split: every crlf line still present once with its \r
	budget := len(hdr) + len(atLine) + 3*len(lines[0])
	got := splitDiff(diff, budget)
	if len(got) < 2 {
		t.Fatalf("got %d chunks, want a split", len(got))
	}
	for _, l := range lines {
		if n := countIn(got, l); n != 1 {
			t.Errorf("line %q appears %d times, want 1", l, n)
		}
	}
}

func TestNoNewlineMarkerStaysRightAfterItsLine(t *testing.T) {
	hdr := fileHeader("n.go")
	lines := numbered("nn", 6)
	marker := "\\ No newline at end of file"
	// marker follows line 2, where a plain line split would cut between them
	body := append(append(append([]string{}, lines[:3]...), marker), lines[3:]...)
	hunk := hunkText("-1,6 +1,6", body...)
	atLine := keepLines(hunk)[0]
	budget := len(hdr) + len(atLine) + 3*len("+nn-00\n")
	got := splitDiff(hdr+hunk, budget)
	if len(got) < 2 {
		t.Fatalf("got %d chunks, want a split", len(got))
	}
	joined := joinChunks(got)
	if !strings.Contains(joined, lines[2]+"\n"+marker+"\n") {
		t.Errorf("marker not directly after %q in %q", lines[2], joined)
	}
	for i, c := range got {
		if strings.HasPrefix(strings.TrimPrefix(c.Text, hdr+atLine), marker) {
			t.Errorf("piece %d starts its body with the marker line", i)
		}
	}
	if n := countIn(got, marker+"\n"); n != 1 {
		t.Errorf("marker appears %d times, want 1", n)
	}
}

func TestSingleOverBudgetLineIsKeptWhole(t *testing.T) {
	// this is the one allowed overshoot: a line longer than the budget
	// cannot be split, so it goes whole into its own piece.
	hdr := fileHeader("l.go")
	long := "+" + strings.Repeat("x", 200)
	atLine := "@@ -1,4 +1,4 @@ ctx\n"
	hunk := atLine + "+s1\n+s2\n" + long + "\n+s3\n"
	budget := len(hdr) + len(atLine) + 20
	got := splitDiff(hdr+hunk, budget)
	if n := countIn(got, long+"\n"); n != 1 {
		t.Fatalf("long line appears %d times, want 1", n)
	}
	for i, c := range got {
		if strings.Contains(c.Text, long) {
			if c.Text != hdr+atLine+long+"\n" {
				t.Errorf("piece %d should hold only the long line: %q", i, c.Text)
			}
		} else if len(c.Text) > budget {
			t.Errorf("piece %d is %d bytes, over budget %d", i, len(c.Text), budget)
		}
	}
	for _, l := range []string{"+s1\n", "+s2\n", "+s3\n"} {
		if n := countIn(got, l); n != 1 {
			t.Errorf("line %q appears %d times, want 1", l, n)
		}
	}
}

func TestTextBeforeFirstFileHeaderStaysAtTopOfFirstChunk(t *testing.T) {
	pre := "From: someone\nSubject: patch\n\n"
	a := fileHeader("a.go") + hunkText("-1 +1", "+a1")
	b := fileHeader("b.go") + hunkText("-1 +1", "+b1")
	got := splitDiff(pre+a+b, len(pre)+len(a)+2)
	if len(got) != 2 {
		t.Fatalf("got %d chunks, want 2", len(got))
	}
	if got[0].Text != pre+a || got[1].Text != b {
		t.Errorf("texts = %q and %q", got[0].Text, got[1].Text)
	}
	if !reflect.DeepEqual(got[0].Files, []string{"a.go"}) {
		t.Errorf("files = %v, want [a.go]", got[0].Files)
	}
}

func TestDiffWithoutFileHeaderSplitsAtLinesOnlyWhenOverBudget(t *testing.T) {
	small := "one\ntwo\n"
	if got := splitDiff(small, 100); len(got) != 1 || got[0].Text != small || len(got[0].Files) != 0 {
		t.Fatalf("small headerless diff changed: %+v", got)
	}
	big := strings.Join(numbered("raw", 10), "\n") + "\n"
	got := splitDiff(big, 3*len("+raw-00\n"))
	if len(got) < 2 {
		t.Fatalf("got %d chunks, want a split", len(got))
	}
	for i, c := range got {
		if len(c.Text) > 3*len("+raw-00\n") || len(c.Files) != 0 {
			t.Errorf("piece %d = %+v", i, c)
		}
	}
	if joinChunks(got) != big {
		t.Error("concatenated text differs from the input")
	}
}
