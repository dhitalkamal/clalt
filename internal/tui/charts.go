package tui

import "strings"

// block glyphs are built from rune values rather than typed literally, so no
// box-drawing/block characters appear in the source.
const (
	fullBlock  = 0x2588 // solid cell
	lightShade = 0x2591 // empty track
	lowEighth  = 0x2580 // add 1..8 -> 0x2581 (1/8 bottom) .. 0x2588 (full)
)

// horizBar renders a filled horizontal bar of the given width for frac in 0..1,
// using solid blocks for the filled part and light shade for the track.
func horizBar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	return strings.Repeat(string(rune(fullBlock)), filled) +
		strings.Repeat(string(rune(lightShade)), width-filled)
}

// meter renders a bracketed bar: "[####----]".
func meter(frac float64, width int) string {
	return "[" + horizBar(frac, width) + "]"
}

// gradedMeter renders a filled bar whose filled portion is colored by threshold
// (green/yellow/red) with a dimmed track for the remainder.
func gradedMeter(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	fillStyle := lipglossFg(gradedColor(frac))
	return fillStyle.Render(strings.Repeat(string(rune(fullBlock)), filled)) +
		dimStyle.Render(strings.Repeat(string(rune(lightShade)), width-filled))
}

// vBarChart renders values as a vertical bar chart `rows` tall. It returns one
// string per row, top row first. Each column is one character wide; sub-cell
// resolution uses eighth blocks so short bars still read.
func vBarChart(values []int64, rows int) []string {
	if rows < 1 {
		rows = 1
	}
	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	lines := make([]string, rows)
	if max == 0 {
		// all empty: blank canvas
		for r := range lines {
			lines[r] = strings.Repeat(" ", len(values))
		}
		return lines
	}
	totalEighths := rows * 8
	// per-column height in eighths
	heights := make([]int, len(values))
	for i, v := range values {
		heights[i] = int(float64(v) / float64(max) * float64(totalEighths))
	}
	// row 0 is the top; the bottom row is rows-1
	for r := 0; r < rows; r++ {
		rowFromBottom := rows - 1 - r
		var sb strings.Builder
		for _, h := range heights {
			cell := h - rowFromBottom*8
			switch {
			case cell <= 0:
				sb.WriteByte(' ')
			case cell >= 8:
				sb.WriteRune(rune(fullBlock))
			default:
				sb.WriteRune(rune(lowEighth + cell))
			}
		}
		lines[r] = sb.String()
	}
	return lines
}

// sparkline renders values as inline block glyphs (U+2581..U+2588), one per value.
func sparkline(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for _, v := range values {
		lvl := 0
		if max > 0 {
			lvl = int(float64(v) / float64(max) * 7)
		}
		b.WriteRune(rune(0x2581 + lvl))
	}
	return b.String()
}

// downsample reduces values to at most width buckets by summing adjacent groups,
// so a long series fits a narrow chart without dropping totals.
func downsample(values []int64, width int) []int64 {
	if width < 1 || len(values) <= width {
		return values
	}
	out := make([]int64, width)
	n := len(values)
	for i, v := range values {
		bucket := i * width / n
		out[bucket] += v
	}
	return out
}
