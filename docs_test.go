package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// the readme is the front door and stays short. the detail lives in docs/, one
// focused file per subject, and every markdown file stays under the soft limit
// for the size of a file. the changelog is left out of the size rule on purpose:
// it only grows.
const softLimit = 300

// the files of the reference. the readme links to each of them.
var reference = []string{
	"docs/commands.md", "docs/sharing.md", "docs/auth.md", "docs/doctor.md",
	"docs/upgrading.md", "docs/migrate.md", "docs/dashboard.md", "docs/office.md",
}

func markdownFiles(t *testing.T) []string {
	t.Helper()
	files := []string{"README.md"}
	err := filepath.WalkDir("docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func TestMarkdownFilesStayUnderTheSoftLimit(t *testing.T) {
	for _, path := range markdownFiles(t) {
		if n := len(lines(t, path)); n >= softLimit {
			t.Errorf("%s has %d lines, the limit is under %d: split it", path, n, softLimit)
		}
	}
}

func TestTheReferenceFilesExistAndTheReadmeLinksToEachOfThem(t *testing.T) {
	readme := strings.Join(lines(t, "README.md"), "\n")
	for _, path := range reference {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s is missing: %v", path, err)
		}
		if !strings.Contains(readme, "]("+path+")") {
			t.Errorf("README.md does not link to %s", path)
		}
	}
}

var link = regexp.MustCompile(`\]\(([^)\s]+)\)`)

func TestRelativeLinksLeadToFiles(t *testing.T) {
	for _, path := range markdownFiles(t) {
		for i, line := range lines(t, path) {
			for _, m := range link.FindAllStringSubmatch(line, -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
					continue
				}
				file, _, _ := strings.Cut(target, "#")
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), file)); err != nil {
					t.Errorf("%s:%d links to %s, which is not there", path, i+1, target)
				}
			}
		}
	}
}

// headingName is what GitHub makes of a heading to link to it: lower case, the
// spaces as hyphens, and nothing but letters, digits, hyphens and underscores.
func headingName(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// a link that ends in #something has to find a heading of that name in the file
// it leads to, or the reader lands at the top of a long page
func TestLinksToAHeadingFindIt(t *testing.T) {
	for _, path := range markdownFiles(t) {
		for i, line := range lines(t, path) {
			for _, m := range link.FindAllStringSubmatch(line, -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				file, fragment, ok := strings.Cut(target, "#")
				if !ok {
					continue
				}
				dest := path
				if file != "" {
					dest = filepath.Join(filepath.Dir(path), file)
				}
				var found bool
				for _, l := range lines(t, dest) {
					if strings.HasPrefix(l, "#") && headingName(strings.TrimLeft(l, "# ")) == fragment {
						found = true
					}
				}
				if !found {
					t.Errorf("%s:%d links to %s, and %s has no such heading", path, i+1, target, dest)
				}
			}
		}
	}
}

func TestMarkdownIsPlainAscii(t *testing.T) {
	for _, path := range append(markdownFiles(t), "CHANGELOG.md") {
		for i, line := range lines(t, path) {
			for _, r := range line {
				if r > 126 {
					t.Errorf("%s:%d has the character %q", path, i+1, r)
					break
				}
			}
		}
	}
}
