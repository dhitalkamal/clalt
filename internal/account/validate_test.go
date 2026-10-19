package account

import (
	"strings"
	"testing"
)

func TestValidateNameAccepts(t *testing.T) {
	valid := []string{
		"work",
		"personal-2",
		"a.b_c",
		"A",
		"contractor.acme",
		// 64 chars is the max allowed length
		strings.Repeat("a", 64),
	}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateNameRejects(t *testing.T) {
	invalid := []string{
		"",        // empty
		"../x",    // path traversal
		"a/b",     // path separator
		"a\\b",    // windows path separator
		"CON",     // windows reserved device name
		"con.txt", // reserved stem with extension
		"nul",     // reserved, lowercase
		"NUL.LOG", // reserved, uppercase with extension
		"COM1",    // reserved comm port
		"LPT9",    // reserved parallel port
		"a:b",     // colon, dangerous on windows (also not in charset)
		".",       // current dir
		"..",      // parent dir
		".hidden", // leading dot
		"-rf",     // leading hyphen, looks like a flag
		"wo rk",   // space not in charset
		"wоrk",    // cyrillic lookalike 'o', not in [A-Za-z0-9._-]
		// 65 chars exceeds the max
		strings.Repeat("a", 65),
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", name)
		}
	}
}

func TestValidateNameErrorMentionsCharset(t *testing.T) {
	err := ValidateName("a/b")
	if err == nil {
		t.Fatal("expected error")
	}
	// the message must be actionable: tell the user what characters are allowed
	got := err.Error()
	if !contains(got, "A-Za-z0-9") && !contains(got, "letters") {
		t.Errorf("error message %q does not describe the allowed charset", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
