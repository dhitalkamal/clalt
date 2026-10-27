package dashboard

// tabStrip pads each name, or brackets the active one in ascii mode. with pad 0
// the brackets add two cells instead of replacing the padding.
func tabStrip(g glyphs, names []string, active, pad, sep int) rich {
	var r rich
	for i, n := range names {
		if i > 0 {
			r = append(r, gap(sep))
		}
		// without brackets the first name starts flush with the text below
		lead := pad
		if i == 0 && !g.ascii {
			lead = 0
		}
		switch {
		case i == active && g.ascii:
			r = append(r, gap(pad-1), bold("["+n+"]"), gap(pad-1))
		case i == active:
			r = append(r, gap(lead), span{toneActive, n}, gap(pad))
		default:
			r = append(r, gap(lead), dim(n), gap(pad))
		}
	}
	return r
}

func tabRow(g glyphs, names []string, active, w int) rich {
	return firstFit(w,
		tabStrip(g, names, active, 1, 2),
		tabStrip(g, names, active, 1, 0),
		tabStrip(g, names, active, 0, 1),
	)
}
