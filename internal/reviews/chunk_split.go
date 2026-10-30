package reviews

import "strings"

// packer fills pieces up to a byte budget. Every piece starts with head
// (the first one also with lead) plus an optional extra prefix.
type packer struct {
	budget int
	lead   string
	head   string
	out    []string
	cur    string
	body   bool // cur holds something beyond its prefix
}

func newPacker(budget int, lead, head string) *packer {
	p := &packer{budget: budget, lead: lead, head: head}
	p.open("")
	return p
}

// open starts a fresh piece: header, then extra (for example an @@ line).
func (p *packer) open(extra string) {
	h := p.head
	if len(p.out) == 0 {
		h = p.lead + h
	}
	p.cur = h + extra
	p.body = false
}

func (p *packer) flush() {
	p.out = append(p.out, p.cur)
	p.cur, p.body = "", false
}

// add appends a unit, starting a new piece (with extra as prefix) when it
// does not fit. A unit that cannot fit even alone is kept whole anyway.
func (p *packer) add(unit, extra string) {
	if p.body && len(p.cur)+len(unit) > p.budget {
		p.flush()
		p.open(extra)
	}
	p.cur += unit
	p.body = true
}

// finish closes the last piece. There is always at least one piece.
func (p *packer) finish() []string {
	if p.body || len(p.out) == 0 {
		p.flush()
	}
	return p.out
}

// splitSection splits one oversized file section at hunk boundaries, and
// oversized hunks at line boundaries. lead goes only on the first piece.
func splitSection(sec, lead string, budget int) []string {
	header, hunks := cutHunks(sec)
	p := newPacker(budget, lead, header)
	for _, h := range hunks {
		if len(p.cur)+len(h) <= budget {
			p.add(h, "")
			continue
		}
		if p.body {
			p.flush()
			p.open("")
		}
		if len(p.cur)+len(h) <= budget {
			p.add(h, "")
			continue
		}
		// hunk too big even alone: pack its lines under the @@ line
		at, rest, _ := strings.Cut(h, "\n")
		at += "\n"
		p.open(at)
		for _, u := range lineUnits(rest) {
			p.add(u, at)
		}
		p.flush()
		p.open("")
	}
	return p.finish()
}

// cutHunks splits a section into its header (everything before the first
// "@@ " line) and its hunks, each starting at an "@@ " line.
func cutHunks(sec string) (string, []string) {
	var header string
	var hunks []string
	for _, line := range splitKeep(sec) {
		switch {
		case strings.HasPrefix(line, "@@ "):
			hunks = append(hunks, line)
		case len(hunks) == 0:
			header += line
		default:
			hunks[len(hunks)-1] += line
		}
	}
	return header, hunks
}

// lineUnits splits text into lines, gluing a "\ No newline at end of
// file" marker onto the line before it so the two never separate.
func lineUnits(s string) []string {
	var units []string
	for _, line := range splitKeep(s) {
		if strings.HasPrefix(line, "\\") && len(units) > 0 {
			units[len(units)-1] += line
			continue
		}
		units = append(units, line)
	}
	return units
}
