package dashboard

import (
	"strconv"
	"strings"
	"testing"
)

func TestCheckLabel(t *testing.T) {
	accts := []Account{{ID: "a1", Label: "robin"}, {ID: "a2", Label: "Morgan"}, {ID: "a3", Label: "harriet"}}
	cases := []struct {
		name, in, id, want, problem string
	}{
		{"plain", "work", "a1", "work", ""},
		{"trimmed", "  work\t", "a1", "work", ""},
		{"empty", "", "a1", "", "Enter a name."},
		{"only spaces", "   ", "a1", "", "Enter a name."},
		{"tab inside", "a\tb", "a1", "a\tb", "No control characters."},
		{"newline inside", "a\nb", "a1", "a\nb", "No control characters."},
		{"escape sequence", "a\x1b[31mb", "a1", "a\x1b[31mb", "No control characters."},
		{"delete", "a\x7fb", "a1", "a\x7fb", "No control characters."},
		{"c1 control", "a\u0085b", "a1", "a\u0085b", "No control characters."},
		{"32 characters", strings.Repeat("a", 32), "a1", strings.Repeat("a", 32), ""},
		{"33 characters", strings.Repeat("a", 33), "a1", strings.Repeat("a", 33), "Use 32 characters or fewer."},
		{"characters, not bytes", strings.Repeat("\u00e9", 32), "a1", strings.Repeat("\u00e9", 32), ""},
		{"duplicate ignoring case", "morgan", "a1", "morgan", "Another account has that name."},
		{"duplicate after trimming", " HARRIET ", "a1", "HARRIET", "Another account has that name."},
		{"own name is not a duplicate", "robin", "a1", "robin", ""},
		{"own name in another case", "ROBIN", "a1", "ROBIN", ""},
		{"another account's name in my case", "Robin", "a2", "Robin", "Another account has that name."},
	}
	for _, c := range cases {
		got, problem := checkLabel(c.in, c.id, accts)
		if got != c.want || problem != c.problem {
			t.Errorf("%s: checkLabel(%q) = %q, %q; want %q, %q", c.name, c.in, got, problem, c.want, c.problem)
		}
	}
}

func TestCheckLabelChecksTheEmptyNameFirst(t *testing.T) {
	if _, problem := checkLabel("\t", "a1", nil); problem != "Enter a name." {
		t.Errorf("a name of only white space is empty, not a control character problem: %q", problem)
	}
}

func TestTheLengthNoteNamesTheLimit(t *testing.T) {
	if !strings.Contains(noteLong, strconv.Itoa(maxLabelLen)) {
		t.Errorf("%q should say %d", noteLong, maxLabelLen)
	}
}
