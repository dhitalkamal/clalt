package reviews

import "strings"

// part is the parsed reply of one chunk run.
type part struct {
	Findings []Finding
	Summary  string
}

// findingKey is what makes two findings the same one.
type findingKey struct {
	File     string
	Line     int
	Severity string
	Title    string
}

// mergeParts joins chunk replies: findings in chunk then reply order with
// duplicates dropped (the first one is kept), summaries of the non-empty
// replies in chunk order separated by a blank line. the joined summary is
// capped like any summary, with maxSummary.
func mergeParts(parts []part) ([]Finding, string) {
	findings := []Finding{}
	seen := map[findingKey]bool{}
	var sums []string
	for _, p := range parts {
		for _, f := range p.Findings {
			k := findingKey{f.File, f.Line, f.Severity, f.Title}
			if seen[k] {
				continue
			}
			seen[k] = true
			findings = append(findings, f)
		}
		if strings.TrimSpace(p.Summary) != "" {
			sums = append(sums, p.Summary)
		}
	}
	return findings, tidy(strings.Join(sums, "\n\n"), maxSummary)
}
