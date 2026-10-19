package dashboard

import (
	"strings"
	"testing"
)

var testDays = []string{"Thu", "Fri", "Sat", "Sun", "Mon", "Tue", "Wed"}

// testReadings is 56 readings, eight a day, ending at 26 percent
func testReadings() []float64 {
	day := []float64{8, 22, 61, 74, 35, 52, 19, 6}
	var out []float64
	for range testDays {
		out = append(out, day...)
	}
	out[len(out)-1] = 26
	return out
}

func TestResample(t *testing.T) {
	vals := []float64{1, 9, 2, 8, 3, 7}
	if got := resample(vals, 3); got[0] != 9 || got[1] != 8 || got[2] != 7 {
		t.Errorf("shrinking keeps the largest value of each bucket, got %v", got)
	}
	if got := resample([]float64{4, 8}, 6); len(got) != 6 || got[0] != 4 || got[5] != 8 {
		t.Errorf("growing repeats the nearest value, got %v", got)
	}
}

func TestChartShape(t *testing.T) {
	for _, g := range []glyphs{asciiGlyphs(), unicodeGlyphs()} {
		for _, w := range []int{27, 43, 54, 67, 120} {
			for h := 3; h <= 6; h++ {
				rows := chartRows(g, testReadings(), testDays, w, h)
				if len(rows) != h+2 {
					t.Fatalf("h=%d: %d rows, want %d", h, len(rows), h+2)
				}
				for i, r := range rows {
					if r.width() > w {
						t.Fatalf("w=%d h=%d row %d is %d wide", w, h, i, r.width())
					}
					if r.width() != rows[0].width() && i < h+1 {
						t.Fatalf("w=%d h=%d row %d is %d wide, row 0 is %d", w, h, i, r.width(), rows[0].width())
					}
				}
				days := plainText(rows[len(rows)-1])
				if strings.Index(days, "Thu") > strings.Index(days, "Wed") || !strings.Contains(days, "Sun") {
					t.Errorf("day labels out of order: %q", days)
				}
			}
		}
	}
}

func TestChartUsesTheGivenDayLabels(t *testing.T) {
	days := []string{"Mon", "Tue", "Wed"}
	rows := chartRows(asciiGlyphs(), []float64{10, 40, 80, 20, 60, 30}, days, 40, 4)
	last := plainText(rows[len(rows)-1])
	for _, d := range days {
		if !strings.Contains(last, d) {
			t.Errorf("axis labels %q should hold %s", last, d)
		}
	}
	if strings.Contains(last, "Thu") || strings.Contains(last, "Sun") {
		t.Errorf("labels come from the caller, got %q", last)
	}
	axis := plainText(rows[len(rows)-2])
	if n := strings.Count(axis, "+"); n != len(days) {
		t.Errorf("axis %q should have a corner and %d day ticks, found %d plus signs", axis, len(days)-1, n)
	}
}

func TestChartAxisLabelsAndBars(t *testing.T) {
	g := asciiGlyphs()
	rows := chartRows(g, testReadings(), testDays, 60, 6)
	if !strings.HasPrefix(plainText(rows[0]), "100%") || !strings.Contains(plainText(rows[3]), " 50%") {
		t.Errorf("the top row is 100%% and the middle 50%%: %q / %q", plainText(rows[0]), plainText(rows[3]))
	}
	if !strings.HasPrefix(plainText(rows[6]), "  0%") {
		t.Errorf("the axis row is labelled 0%%: %q", plainText(rows[6]))
	}
	// a 61 percent reading fills the bottom rows green and a 91 reading is red
	colored := chartRows(g, []float64{61, 91, 61, 91, 61, 91, 61}, testDays, 40, 6)
	sawGood, sawBad := false, false
	for _, r := range colored {
		for _, sp := range r {
			sawGood = sawGood || sp.t == toneGood
			sawBad = sawBad || sp.t == toneBad
		}
	}
	if !sawGood || !sawBad {
		t.Errorf("columns take the usage tone of their value (good %v, bad %v)", sawGood, sawBad)
	}
}

func TestChartWithNothingToDraw(t *testing.T) {
	if chartRows(asciiGlyphs(), nil, testDays, 60, 5) != nil {
		t.Error("no readings, no chart")
	}
	if chartRows(asciiGlyphs(), testReadings(), nil, 60, 5) != nil {
		t.Error("no day labels, no chart")
	}
}
