package account

import (
	"fmt"
	"regexp"
	"strings"
)

// nameRe is the full allowed charset for an account name: ASCII letters,
// digits, dot, underscore, hyphen, 1-64 chars. Names get joined directly into
// filesystem paths (the account folder), so anything outside this set (path
// separators, colons, nulls, ..) must never reach a name in the first place.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// windowsReservedNames are device names Windows treats specially regardless
// of extension (CON, con.txt, and CON.anything all mean the same device).
// Checked case-insensitively against the part of the name before the first
// dot.
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidateName rejects account names that are not safe to use as a bare
// filesystem path segment: names come straight from CLI args and are joined
// verbatim into the account folder path. Without this check a name like
// "../../../../tmp/x" would point an account folder at an attacker-chosen
// directory.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("account name %q: only letters, digits, '.', '_', '-' are allowed, 1-64 chars", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("account name %q is not allowed", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("account name %q must not start with '.'", name)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("account name %q must not start with '-' (would look like a flag)", name)
	}
	stem := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		stem = name[:i]
	}
	if windowsReservedNames[strings.ToLower(stem)] {
		return fmt.Errorf("account name %q is a reserved device name on Windows", name)
	}
	return nil
}
