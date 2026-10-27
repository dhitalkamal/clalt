package dashboard

import (
	"strings"
	"testing"
)

func TestTableAlignsColumns(t *testing.T) {
	cols := []column{{"day", false}, {"cost", true}}
	rows := [][]rich{
		{{txt("Wed 15")}, {txt("~$2.14")}},
		{{txt("Mon 6")}, {txt("~$12.55")}},
	}
	head, body := table(cols, rows, 2)
	if got, want := plainText(head), "day"+strings.Repeat(" ", 8)+"cost"; got != want {
		t.Errorf("head = %q, want %q", got, want)
	}
	want := []string{"Wed 15   ~$2.14", "Mon 6   ~$12.55"}
	for i, w := range want {
		if got := plainText(body[i]); got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
	if head.width() != body[0].width() || tableWidth(cols, rows, 2) != head.width() {
		t.Error("head, rows and tableWidth should agree")
	}
	if head[0].t != toneDim {
		t.Error("header cells are dim")
	}
}

func TestThumbRows(t *testing.T) {
	if thumbRows(10, 10, 0) != nil || thumbRows(5, 10, 0) != nil {
		t.Error("no thumb when everything fits")
	}
	for total := 11; total < 60; total += 7 {
		for vh := 3; vh < total; vh += 4 {
			for off := 0; off <= total-vh; off++ {
				rows := thumbRows(total, vh, off)
				n := 0
				for _, on := range rows {
					if on {
						n++
					}
				}
				if len(rows) != vh || n < 1 {
					t.Fatalf("total %d vh %d off %d: %d rows, %d marked", total, vh, off, len(rows), n)
				}
			}
			top, bottom := thumbRows(total, vh, 0), thumbRows(total, vh, total-vh)
			if !top[0] || !bottom[vh-1] {
				t.Errorf("thumb should touch the top at offset 0 and the bottom at the end (total %d vh %d)", total, vh)
			}
		}
	}
}

func TestTabRowFitsOrIsClipped(t *testing.T) {
	for _, g := range []glyphs{asciiGlyphs(), unicodeGlyphs()} {
		for w := 30; w <= 80; w++ {
			for active := range tabNames {
				row := tabRow(g, tabNames, active, w)
				if row.width() > w {
					t.Fatalf("w=%d active=%d: tab row is %d wide", w, active, row.width())
				}
			}
		}
		full := tabRow(g, tabNames, 0, 54)
		for _, n := range tabNames {
			if !strings.Contains(plainText(full), n) {
				t.Errorf("roomy tab row lost %s", n)
			}
		}
	}
}

func TestActiveTabIsBracketedInAsciiOnly(t *testing.T) {
	a, u := asciiGlyphs(), unicodeGlyphs()
	for active, name := range tabNames {
		if got := plainText(tabRow(a, tabNames, active, 90)); !strings.Contains(got, "["+name+"]") {
			t.Errorf("ascii tab %d is not bracketed: %q", active, got)
		}
		if got := plainText(tabRow(u, tabNames, active, 90)); strings.Contains(got, "[") {
			t.Errorf("unicode tabs mark the active one with style, not brackets: %q", got)
		}
	}
	// moving the active tab never shifts the others
	w0 := tabRow(a, tabNames, 0, 90).width()
	for active := range tabNames {
		if w := tabRow(a, tabNames, active, 90).width(); w != w0 {
			t.Errorf("active=%d changes the tab row width from %d to %d", active, w0, w)
		}
	}
}

func TestTabStripWithoutPaddingBracketsAddTwoCells(t *testing.T) {
	a := asciiGlyphs()
	plain := tabStrip(a, subNames, 0, 0, 2)
	if got := plainText(plain); got != "[Days]  Projects  Models" {
		t.Errorf("sub tabs = %q", got)
	}
}
