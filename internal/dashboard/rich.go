package dashboard

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// lines are built from spans of plain text so width math, wrapping and
// clipping never have to parse escape codes
type span struct {
	t tone
	s string
}

type rich []span

func txt(s string) span  { return span{toneText, s} }
func dim(s string) span  { return span{toneDim, s} }
func good(s string) span { return span{toneGood, s} }
func warn(s string) span { return span{toneWarn, s} }
func bad(s string) span  { return span{toneBad, s} }
func bold(s string) span { return span{toneBold, s} }

func gap(n int) span {
	return span{toneText, strings.Repeat(" ", max(n, 0))}
}

func cat(parts ...rich) rich {
	var out rich
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func clampInt(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

func (r rich) width() int {
	n := 0
	for _, sp := range r {
		n += ansi.StringWidth(sp.s)
	}
	return n
}

func (r rich) clip(w int) rich {
	var out rich
	for _, sp := range r {
		if w <= 0 {
			break
		}
		sw := ansi.StringWidth(sp.s)
		if sw <= w {
			out = append(out, sp)
			w -= sw
			continue
		}
		out = append(out, span{sp.t, ansi.Truncate(sp.s, w, "")})
		break
	}
	return out
}

func (r rich) fit(w int) rich {
	r = r.clip(w)
	if d := w - r.width(); d > 0 {
		r = append(r[:len(r):len(r)], gap(d))
	}
	return r
}

// draw merges neighbours of one tone into a single escape
func (p palette) draw(r rich) string {
	var sb, run strings.Builder
	var cur tone
	for _, sp := range r {
		if sp.s == "" {
			continue
		}
		if run.Len() > 0 && sp.t != cur {
			sb.WriteString(p.apply(cur, run.String()))
			run.Reset()
		}
		cur = sp.t
		run.WriteString(sp.s)
	}
	if run.Len() > 0 {
		sb.WriteString(p.apply(cur, run.String()))
	}
	return sb.String()
}

// words splits into alternating runs of spaces and non-spaces
func words(s string) []string {
	var out []string
	start := 0
	for i := 1; i <= len(s); i++ {
		if i == len(s) || (s[i] == ' ') != (s[start] == ' ') {
			out = append(out, s[start:i])
			start = i
		}
	}
	return out
}

func trimEnd(r rich) rich {
	out := append(rich(nil), r...)
	for len(out) > 0 {
		t := strings.TrimRight(out[len(out)-1].s, " ")
		if t != "" {
			out[len(out)-1].s = t
			break
		}
		out = out[:len(out)-1]
	}
	return out
}

// wrap keeps each span's tone and drops the spaces at a break
func wrap(r rich, w int) []rich {
	w = max(w, 1)
	var lines []rich
	var cur rich
	curW := 0
	flush := func() {
		lines = append(lines, trimEnd(cur))
		cur, curW = nil, 0
	}
	place := func(t tone, tok string) {
		tw := ansi.StringWidth(tok)
		for curW+tw > w {
			if curW > 0 {
				flush()
				continue
			}
			head := ansi.Truncate(tok, w, "")
			if head == "" {
				return
			}
			cur = append(cur, span{t, head})
			curW = ansi.StringWidth(head)
			flush()
			tok = ansi.TruncateLeft(tok, w, "")
			if tw = ansi.StringWidth(tok); tw == 0 {
				return
			}
		}
		cur = append(cur, span{t, tok})
		curW += tw
	}
	for _, sp := range r {
		if sp.s == "" {
			continue
		}
		for _, tok := range words(sp.s) {
			if tok[0] != ' ' {
				place(sp.t, tok)
				continue
			}
			tw := ansi.StringWidth(tok)
			switch {
			case curW == 0 && len(lines) > 0:
			case curW+tw > w:
				flush()
			default:
				cur = append(cur, span{sp.t, tok})
				curW += tw
			}
		}
	}
	if len(cur) > 0 {
		flush()
	}
	return lines
}

// hang wraps body beside a fixed width label, indenting later lines under it
func hang(label rich, labelW int, body rich, w int) []rich {
	return beside(label, labelW, wrap(body, max(w-labelW, 1)))
}

// beside puts the label before the first line and indents the rest under it
func beside(label rich, labelW int, lines []rich) []rich {
	if len(lines) == 0 {
		return []rich{label.fit(labelW)}
	}
	out := make([]rich, len(lines))
	for i, l := range lines {
		if i == 0 {
			out[i] = cat(label.fit(labelW), l)
		} else {
			out[i] = cat(rich{gap(labelW)}, l)
		}
	}
	return out
}

// flow packs whole items onto lines, sep spaces apart, so a number never parts
// from its unit. an item wider than a line wraps on its own.
func flow(items []rich, sep, w int) []rich {
	var lines []rich
	var cur rich
	for _, it := range items {
		switch iw := it.width(); {
		case iw == 0:
		case iw > w:
			if len(cur) > 0 {
				lines, cur = append(lines, cur), nil
			}
			lines = append(lines, wrap(it, w)...)
		case len(cur) == 0:
			cur = it
		case cur.width()+sep+iw <= w:
			cur = cat(cur, rich{gap(sep)}, it)
		default:
			lines, cur = append(lines, cur), it
		}
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// firstFit falls back to the last candidate, clipped, when none fit
func firstFit(w int, cands ...rich) rich {
	for _, c := range cands {
		if c.width() <= w {
			return c
		}
	}
	return cands[len(cands)-1].clip(w)
}

func alignRight(left, right rich, w int) rich {
	return cat(left.fit(max(w-right.width(), 0)), right)
}

func (r rich) ellipsize(w int, ell string) rich {
	if r.width() <= w {
		return r
	}
	return append(r.clip(max(w-ansi.StringWidth(ell), 0)), dim(ell))
}

// prompt alternates dim text and bold key names: "Press", "s", "to sign in"
func prompt(parts ...string) rich {
	var r rich
	for i, p := range parts {
		if i%2 == 0 {
			r = append(r, dim(p))
		} else {
			r = append(r, bold(p))
		}
	}
	return r
}
