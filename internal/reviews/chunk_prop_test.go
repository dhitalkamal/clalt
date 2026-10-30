package reviews

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// genFile is one generated file section plus what the guard needs to know.
type genFile struct {
	name    string
	section string
	header  []string // header lines, each repeated once per piece
	atLines []string // @@ lines, may be repeated for split hunks
	content []string // content lines, must appear exactly once overall
}

// genDiff builds a random diff with unique content lines (file index and
// line number are embedded in every line).
func genDiff(r *rand.Rand) []genFile {
	var files []genFile
	for fi, n := 0, 1+r.Intn(8); fi < n; fi++ {
		f := genFile{name: fmt.Sprintf("dir/f%d.go", fi)}
		hdr := fileHeader(f.name)
		f.header = keepLines(hdr)
		var b strings.Builder
		b.WriteString(hdr)
		hunks := 1 + r.Intn(5)
		if r.Intn(4) == 0 {
			hunks = 1 // single-hunk files make oversized hunks likely
		}
		maxLines := 12
		if r.Intn(3) == 0 {
			maxLines = 90
		}
		for h := 0; h < hunks; h++ {
			at := fmt.Sprintf("@@ -%d,9 +%d,9 @@ f%d-h%d\n", h*100, h*100, fi, h)
			f.atLines = append(f.atLines, at)
			b.WriteString(at)
			for l, m := 0, 1+r.Intn(maxLines); l < m; l++ {
				line := fmt.Sprintf("+f%d-h%d-l%d %s\n", fi, h, l, strings.Repeat("p", r.Intn(30)))
				f.content = append(f.content, line)
				b.WriteString(line)
			}
		}
		f.section = b.String()
		files = append(files, f)
	}
	return files
}

// the guard: nothing is lost or duplicated, except the file header lines
// and @@ lines that are repeated on purpose for files that were split.
// those are the one allowed duplication, counted only for split files.
func TestRandomDiffsAreChunkedWithoutLossOrOvershoot(t *testing.T) {
	r := rand.New(rand.NewSource(20240607))
	for iter := 0; iter < 300; iter++ {
		files := genDiff(r)
		var in strings.Builder
		for _, f := range files {
			in.WriteString(f.section)
		}
		budget := 300 + r.Intn(1200) // always fits header + @@ + longest line
		got := splitDiff(in.String(), budget)
		ctx := fmt.Sprintf("iter %d budget %d", iter, budget)

		if len(got) == 0 {
			t.Fatalf("%s: no chunks", ctx)
		}
		for i, c := range got {
			if len(c.Text) > budget {
				t.Fatalf("%s: chunk %d is %d bytes, over budget", ctx, i, len(c.Text))
			}
			if !strings.HasPrefix(c.Text, "diff --git ") || !strings.HasSuffix(c.Text, "\n") {
				t.Fatalf("%s: chunk %d is not a valid diff piece: %q", ctx, i, c.Text)
			}
		}

		// chunk indices that list each file, in order
		where := map[string][]int{}
		var order []string
		for i, c := range got {
			seen := map[string]bool{}
			for _, p := range c.Files {
				if seen[p] {
					t.Fatalf("%s: chunk %d lists %s twice", ctx, i, p)
				}
				seen[p] = true
				if len(where[p]) == 0 {
					order = append(order, p)
				}
				where[p] = append(where[p], i)
			}
		}
		if len(order) != len(files) {
			t.Fatalf("%s: files listed %v, want %d files", ctx, order, len(files))
		}

		for fi, f := range files {
			if order[fi] != f.name {
				t.Fatalf("%s: file order %v", ctx, order)
			}
			idx := where[f.name]
			for k := 1; k < len(idx); k++ {
				if idx[k] != idx[k-1]+1 {
					t.Fatalf("%s: %s pieces not in consecutive chunks: %v", ctx, f.name, idx)
				}
			}
			pieces := len(idx)
			if len(f.section) <= budget && pieces != 1 {
				t.Fatalf("%s: %s fits the budget but was split into %d", ctx, f.name, pieces)
			}
			for _, l := range f.content {
				if n := countIn(got, l); n != 1 {
					t.Fatalf("%s: %s line %q appears %d times", ctx, f.name, l, n)
				}
			}
			for _, l := range f.header {
				if n := countIn(got, l); n != pieces {
					t.Fatalf("%s: header line %q appears %d times, want %d", ctx, l, n, pieces)
				}
			}
			for _, l := range f.atLines {
				if n := countIn(got, l); n < 1 || n > pieces {
					t.Fatalf("%s: @@ line %q appears %d times, want 1..%d", ctx, l, n, pieces)
				}
			}
			if pieces == 1 && !strings.Contains(joinChunks(got), f.section) {
				t.Fatalf("%s: unsplit file %s was altered", ctx, f.name)
			}
		}

		// content lines come back in their original order
		var want, have []string
		for _, f := range files {
			want = append(want, f.content...)
		}
		for _, l := range keepLines(joinChunks(got)) {
			if strings.HasPrefix(l, "+f") {
				have = append(have, l)
			}
		}
		if strings.Join(have, "") != strings.Join(want, "") {
			t.Fatalf("%s: content lines reordered or changed", ctx)
		}
	}
}
