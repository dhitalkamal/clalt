package reviews

import "strings"

// chunk is one slice of a unified diff small enough to review on its own.
// Files lists the file paths the slice touches, in diff order.
type chunk struct {
	Files []string
	Text  string
}

// chunkBytes is the default diff budget of one claude run. the whole prompt
// (about 1.5 KB of template plus the chunk) stays near 100 KB, roughly 25-30k
// tokens. that leaves most of the context window for the files the reviewer
// reads with Read, Grep and Glob and for its reply, and keeps one run well
// inside reviewTimeout. a typical pull request is one chunk and is reviewed
// exactly as before.
const chunkBytes = 100 * 1024

const fileMarker = "diff --git "

// splitDiff cuts a unified diff into chunks of at most budget bytes where
// it can. File sections are packed greedily; a section over budget is split
// at hunk boundaries (header repeated), and a hunk over budget at line
// boundaries (header and @@ line repeated). No line is dropped or cut, so
// a single line longer than the budget goes whole into its own piece.
// There is always at least one chunk.
func splitDiff(diff string, budget int) []chunk {
	lead, sections := cutSections(diff)
	if len(sections) == 0 {
		// no file header at all: plain line packing, no file names
		p := newPacker(budget, "", "")
		for _, u := range lineUnits(lead) {
			p.add(u, "")
		}
		var out []chunk
		for _, t := range p.finish() {
			out = append(out, chunk{Text: t})
		}
		return out
	}

	var out []chunk
	var cur chunk
	flush := func() {
		if cur.Text != "" {
			out = append(out, cur)
		}
		cur = chunk{}
	}
	for i, sec := range sections {
		name := sectionName(sec)
		prefix := ""
		if i == 0 {
			// text before the first file header rides on the first chunk
			prefix = lead
		}
		whole := prefix + sec
		if len(cur.Text)+len(whole) <= budget {
			cur.Text += whole
			cur.Files = append(cur.Files, name)
			continue
		}
		flush()
		if len(whole) <= budget {
			cur = chunk{Files: []string{name}, Text: whole}
			continue
		}
		for _, t := range splitSection(sec, prefix, budget) {
			out = append(out, chunk{Files: []string{name}, Text: t})
		}
	}
	flush()
	return out
}

// cutSections returns the text before the first file header and then one
// string per file section, each starting at its "diff --git " line.
func cutSections(diff string) (string, []string) {
	var lead string
	var sections []string
	for _, line := range splitKeep(diff) {
		switch {
		case strings.HasPrefix(line, fileMarker):
			sections = append(sections, line)
		case len(sections) == 0:
			lead += line
		default:
			sections[len(sections)-1] += line
		}
	}
	return lead, sections
}

// sectionName reads the new-side path from the section's first line:
// "diff --git a/old b/new" gives "new".
func sectionName(sec string) string {
	first, _, _ := strings.Cut(sec, "\n")
	rest := strings.TrimSuffix(strings.TrimPrefix(first, fileMarker), "\r")
	if i := strings.LastIndex(rest, " b/"); i >= 0 {
		return rest[i+3:]
	}
	return rest
}

// splitKeep splits s into lines, each keeping its own terminator.
func splitKeep(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.SplitAfter(s, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
