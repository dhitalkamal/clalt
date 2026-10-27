package dashboard

import (
	"math"
	"strings"
)

// halfCells never rounds a nonzero percent down to an empty bar
func halfCells(pct float64, w int) int {
	n := int(math.Round(pct / 100 * float64(w*2)))
	if pct > 0 && n == 0 {
		n = 1
	}
	return clampInt(n, 0, w*2)
}

func bar(g glyphs, pct float64, w int, fill tone) rich {
	n := halfCells(pct, w)
	full, half := n/2, n%2
	var r rich
	if full > 0 {
		r = append(r, span{fill, strings.Repeat(g.full, full)})
	}
	if half > 0 {
		r = append(r, span{fill, g.half})
	}
	if rest := w - full - half; rest > 0 {
		r = append(r, span{toneDim, strings.Repeat(g.track, rest)})
	}
	return r
}

func usageBar(g glyphs, pct float64, w int) rich {
	return bar(g, pct, w, levelTone(pct))
}

// shares are not a warning, so their bar stays neutral
func shareBar(g glyphs, pct float64, w int) rich {
	return bar(g, pct, w, toneText)
}

func rampIndex(v, hi float64) int {
	if hi <= 0 {
		return 0
	}
	return clampInt(int(math.Round(v/hi*7)), 0, 7)
}

func spark(g glyphs, vals []float64) rich {
	hi := 0.0
	for _, v := range vals {
		hi = max(hi, v)
	}
	var sb strings.Builder
	for _, v := range vals {
		sb.WriteString(g.ramp[rampIndex(v, hi)])
	}
	return rich{txt(sb.String())}
}
