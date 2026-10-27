package cli

import (
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/migrate"
)

// renderMigrate shows paths and counts, never what is inside a file, and cleans
// every name so none can forge a line.
func renderMigrate(rep migrate.Report) string {
	var b strings.Builder
	if len(rep.Profiles) == 0 {
		fmt.Fprintf(&b, "no profile folders found in %s\n", clean(rep.Source))
		return b.String()
	}
	if rep.DryRun {
		fmt.Fprintf(&b, "dry run, nothing is written. profile history would go into %s\n\n", clean(rep.Dest))
	} else {
		fmt.Fprintf(&b, "migrating profile history into %s\n\n", clean(rep.Dest))
	}

	width := 0
	for _, p := range rep.Profiles {
		width = max(width, len(clean(p.Name)))
	}
	detail := strings.Repeat(" ", width+4)
	for _, p := range rep.Profiles {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, clean(p.Name), profileLine(p, rep.DryRun))
		for _, c := range p.Conflicts {
			fmt.Fprintf(&b, "%sconflict: %s (%s)\n", detail, clean(c.Path), reasonText(c.Reason))
		}
		for _, e := range p.SkippedEntries {
			fmt.Fprintf(&b, "%sskipped: %s (%s)\n", detail, clean(e.Path), skipText(e))
		}
		for _, f := range p.Failed {
			// errors.Join separates its errors with a newline, which clean would
			// show as \x0a, so the lines are joined first.
			msg := strings.ReplaceAll(f.Error, "\n", "; ")
			fmt.Fprintf(&b, "%sfailed %s: %s: %s\n", detail, f.Kind, clean(f.Path), clean(msg))
		}
	}

	fmt.Fprintf(&b, "\ntotal: %s\n", tally(rep.Total, rep.DryRun))
	b.WriteString(footer(rep))
	return b.String()
}

// footer says what to know next. every line is true for the case it covers and
// only appears then.
func footer(rep migrate.Report) string {
	t := rep.Total
	var b strings.Builder
	if rep.DryRun {
		b.WriteString("nothing was written. a dry run does not check free space or write permission.\n")
		if t.Failed == 0 {
			b.WriteString("run 'clalt migrate' to copy for real.\n")
		} else {
			b.WriteString("the dry run found failures, see the failed list above.\n")
		}
		return b.String()
	}
	b.WriteString("the profile folders were left as they were: this only copies.\n")
	if t.Conflicts > 0 {
		b.WriteString("a conflict means the shared folder already holds a different file at that path. the profile's version is only in the profile folder, so keep it.\n")
	}
	if t.Failed > 0 {
		b.WriteString("failed entries are listed above with their reasons.\n")
	}
	if t.Linked > 0 {
		b.WriteString("links are never followed, so what they lead to was not copied.\n")
	}
	if t.InPlace > 0 {
		verb := "were"
		if t.InPlace == 1 {
			verb = "was"
		}
		fmt.Fprintf(&b, "%s %s written in place because the destination has no hard links. that is not atomic: a kill part way can leave a partial file under its real name.\n",
			count(t.InPlace, "file"), verb)
	}
	return b.String()
}

func profileLine(p migrate.Profile, dryRun bool) string {
	switch p.Status {
	case migrate.StatusShared:
		return "already shared"
	case migrate.StatusLink:
		return linkLine(p)
	case migrate.StatusFolder:
		return tally(migrate.Totals{
			Copied:       p.Copied,
			Bytes:        p.Bytes,
			InPlace:      p.InPlace,
			AlreadyThere: p.AlreadyThere,
			Conflicts:    len(p.Conflicts),
			Skipped:      p.Skipped,
			Failed:       len(p.Failed),
		}, dryRun)
	default:
		return "nothing to copy"
	}
}

func linkLine(p migrate.Profile) string {
	what, ending := "projects", "not copied"
	if p.LinkOf == migrate.LinkOfProfile {
		what, ending = "profile folder", "not scanned"
	}
	if p.Error != "" {
		return fmt.Sprintf("%s is a link to %s that cannot be resolved (%s), %s", what, clean(p.Target), clean(p.Error), ending)
	}
	return fmt.Sprintf("%s is a link to %s, %s", what, clean(p.Target), ending)
}

func reasonText(r migrate.Reason) string {
	switch r {
	case migrate.ReasonInTheWay:
		return "a folder, link or special file is in the way"
	case migrate.ReasonShorter:
		return "the shared file is only the start of this one"
	}
	return "the shared folder holds a different file"
}

func skipText(e migrate.Skip) string {
	switch e.Kind {
	case "link":
		if e.Target == "" {
			return "link"
		}
		return "link to " + clean(e.Target)
	case "pipe", "socket", "device":
		return e.Kind
	case "irregular":
		return "irregular file"
	}
	return "special file"
}

// tally words the counters, leaving out the ones that are zero.
func tally(t migrate.Totals, dryRun bool) string {
	verb := "copied"
	if dryRun {
		verb = "would copy"
	}
	parts := []string{fmt.Sprintf("%s %s (%s)", verb, count(t.Copied, "file"), humanBytes(t.Bytes))}
	if t.InPlace > 0 {
		parts = append(parts, fmt.Sprintf("%d written in place", t.InPlace))
	}
	if t.AlreadyThere > 0 {
		parts = append(parts, fmt.Sprintf("%d already there", t.AlreadyThere))
	}
	if t.Conflicts > 0 {
		parts = append(parts, count(t.Conflicts, "conflict"))
	}
	if t.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (links or special files)", t.Skipped))
	}
	if t.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", t.Failed))
	}
	if t.Linked > 0 {
		parts = append(parts, count(t.Linked, "link")+" not followed")
	}
	return strings.Join(parts, ", ")
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
