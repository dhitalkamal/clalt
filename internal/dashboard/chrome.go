package dashboard

// low priority hints are dropped first when the footer is too long
type hint struct {
	key, desc string
	pri       int
}

var normalHints = []hint{
	{"enter", "run", 7}, {"s", "sign in", 6}, {"e", "budget", 5}, {"r", "rename", 3}, {"d", "sign out", 2}, {"D", "delete", 0},
	{"tab", "pane", 9}, {"[ ] or 1-5", "view", 8}, {"/", "find", 4}, {"a", "ascii", 1}, {"?", "keys", 11}, {"q", "quit", 10},
}

// keptHints is the normal footer with the way out of a kept filter in front
func keptHints() []hint { return append([]hint{{"esc", "clear filter", 10}}, normalHints...) }

var filterHints = []hint{
	{"enter", "keep filter", 3}, {"esc", "clear", 4}, {"up down", "move", 1}, {"ctrl+c", "quit", 2},
}

var drillHints = []hint{
	{"esc", "back", 10}, {"tab", "pane", 3}, {"[ ] or 1-5", "view", 4}, {"?", "keys", 8}, {"q", "quit", 7},
}

var helpHints = []hint{{"esc", "close", 3}, {"?", "close", 2}, {"q", "quit", 1}}

// headerLine puts the first candidate that fits flush right beside the name
func headerLine(w int, cands ...rich) rich {
	left := rich{gap(margin), bold("clalt")}
	for _, c := range cands {
		if left.width()+2+c.width() <= w-margin {
			return alignRight(left, c, w-margin)
		}
	}
	return left
}

func hintsLine(items []hint, w int) rich {
	build := func(its []hint, sepW int) rich {
		r := rich{gap(2)}
		for i, h := range its {
			if i > 0 {
				r = append(r, gap(sepW))
			}
			r = append(r, bold(h.key), gap(1), dim(h.desc))
		}
		return r
	}
	for _, sepW := range []int{3, 2} {
		if r := build(items, sepW); r.width() <= w {
			return r
		}
	}
	keep := append([]hint(nil), items...)
	for len(keep) > 1 {
		lo := 0
		for i, h := range keep {
			if h.pri < keep[lo].pri {
				lo = i
			}
		}
		keep = append(keep[:lo], keep[lo+1:]...)
		if r := build(keep, 2); r.width() <= w {
			return r
		}
	}
	return build(keep, 2).clip(w)
}
