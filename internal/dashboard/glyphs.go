package dashboard

import "charm.land/lipgloss/v2"

// every drawn symbol lives here, built from code points and lipgloss borders,
// so no source file has a literal box character
type glyphs struct {
	ascii    bool
	border   lipgloss.Border
	axis     lipgloss.Border
	thumb    string
	cursor   string
	dot      string
	full     string
	half     string
	track    string
	ramp     [8]string // sparkline and chart levels, lowest first
	ellipsis string
}

func cp(r rune) string { return string(r) }

func unicodeGlyphs() glyphs {
	g := glyphs{
		border:   lipgloss.RoundedBorder(),
		axis:     lipgloss.NormalBorder(),
		thumb:    lipgloss.ThickBorder().Right,
		cursor:   cp(0x203a), // single right angle quote
		dot:      cp(0x25cf), // black circle
		full:     cp(0x2588), // full block
		half:     cp(0x258c), // left half block
		track:    cp(0x2591), // light shade
		ellipsis: cp(0x2026),
	}
	// the eight lower block elements are consecutive code points
	for i := range g.ramp {
		g.ramp[i] = cp(rune(0x2581 + i))
	}
	return g
}

func asciiGlyphs() glyphs {
	return glyphs{
		ascii:    true,
		border:   lipgloss.ASCIIBorder(),
		axis:     lipgloss.ASCIIBorder(),
		thumb:    "#",
		cursor:   ">",
		dot:      "*",
		full:     "#",
		half:     "=",
		track:    "-",
		ramp:     [8]string{"_", ".", ":", "-", "=", "+", "*", "#"},
		ellipsis: "...",
	}
}

func glyphsFor(ascii bool) glyphs {
	if ascii {
		return asciiGlyphs()
	}
	return unicodeGlyphs()
}
