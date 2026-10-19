package dashboard

import (
	"math"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// resample keeps the peak of each bucket when shrinking so spikes survive
func resample(vals []float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		lo := i * len(vals) / n
		hi := max((i+1)*len(vals)/n, lo+1)
		for _, v := range vals[lo:min(hi, len(vals))] {
			out[i] = max(out[i], v)
		}
	}
	return out
}

// chartRows draws percent readings as columns grouped under one label per day,
// h rows of bars plus the axis and the day labels. the width is filled, so day
// groups can differ by a column.
func chartRows(g glyphs, vals []float64, days []string, w, h int) []rich {
	if len(vals) == 0 || len(days) == 0 {
		return nil
	}
	const labelW = 4
	prefix := labelW + 2
	d := len(days)
	n := clampInt(w-prefix, 3*d, 10*d)
	cols := resample(vals, n)
	edge := func(j int) int { return j * n / d }
	label := func(r int) string {
		switch r {
		case h - 1:
			return "100%"
		case (h - 1) / 2:
			return "50%"
		}
		return ""
	}
	pad := func(s string) string { return strings.Repeat(" ", labelW-len(s)) + s }

	rows := make([]rich, 0, h+2)
	for r := h - 1; r >= 0; r-- {
		line := rich{dim(pad(label(r))), gap(1), dim(g.axis.Left)}
		for _, v := range cols {
			level := int(math.Round(v / 100 * float64(h*8)))
			fill := clampInt(level-r*8, 0, 8)
			if fill == 0 {
				line = append(line, gap(1))
				continue
			}
			line = append(line, span{levelTone(v), g.ramp[fill-1]})
		}
		rows = append(rows, line)
	}

	ticks := map[int]bool{}
	for j := 1; j < d; j++ {
		ticks[edge(j)] = true
	}
	axis := rich{dim(pad("0%")), gap(1), dim(g.axis.BottomLeft)}
	for i := range cols {
		glyph := g.axis.Bottom
		if ticks[i] {
			glyph = g.axis.MiddleBottom
		}
		axis = append(axis, dim(glyph))
	}
	labels := rich{gap(prefix)}
	for j, name := range days {
		gw := edge(j+1) - edge(j)
		name = ansi.Truncate(name, gw, "")
		off := (gw - ansi.StringWidth(name)) / 2
		labels = append(labels, gap(off), dim(name), gap(gw-ansi.StringWidth(name)-off))
	}
	return append(rows, axis, labels)
}
