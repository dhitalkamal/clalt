package claudecli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// MinVersion is the oldest claude clalt will drive. 2.1.301 fixed sign-outs when `claude auth
// status` exited in the middle of a token refresh, and clalt runs that command
// often.
const MinVersion = "2.1.301"

var (
	versionMu sync.Mutex
	versionOK = map[string]bool{} // binary path -> already checked
)

var versionRe = regexp.MustCompile(`^v?(\d+(?:\.\d+)+)\b`)

// Version returns the dotted version claude reports, from output such as "2.1.303
// (Claude Code)". like Status and Logout it gives up after authTimeout when the
// context has no deadline.
func Version(ctx context.Context) (string, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()
	env, _ := stripAuth(os.Environ())
	res, err := run(ctx, env, "--version")
	if err != nil {
		return "", fmt.Errorf("claude --version: %w", err)
	}
	if res.signal != "" || res.code != 0 {
		return "", res.failure("--version", res.ended())
	}
	m := versionRe.FindStringSubmatch(strings.TrimSpace(string(res.stdout)))
	if m == nil {
		return "", fmt.Errorf("claude --version: unexpected output %q", firstLine(res.stdout))
	}
	return m[1], nil
}

// ensureVersion refuses a claude older than MinVersion, or one whose version
// cannot be read. it asks once per binary per process: the question is cheap,
// but Status runs often. a failed check is not remembered, so a claude that was
// being upgraded gets another chance.
func ensureVersion(ctx context.Context) error {
	bin, err := Binary()
	if err != nil {
		return err
	}
	versionMu.Lock()
	checked := versionOK[bin]
	versionMu.Unlock()
	if checked {
		return nil
	}
	v, err := Version(ctx)
	if err != nil {
		return fmt.Errorf("cannot tell which version of claude this is, clalt needs %s or newer: %w", MinVersion, err)
	}
	cmp, err := CompareVersions(v, MinVersion)
	if err != nil {
		return fmt.Errorf("claude reports version %q, which cannot be compared with the minimum %s: %w", v, MinVersion, err)
	}
	if cmp < 0 {
		return fmt.Errorf("claude %s is older than the minimum %s, update claude", v, MinVersion)
	}
	versionMu.Lock()
	versionOK[bin] = true
	versionMu.Unlock()
	return nil
}

// CompareVersions returns -1, 0 or 1 as a is older than, equal to or newer than b. parts
// compare as numbers, so 2.1.10 is newer than 2.1.9, and a missing part counts
// as zero.
func CompareVersions(a, b string) (int, error) {
	pa, err := versionParts(a)
	if err != nil {
		return 0, err
	}
	pb, err := versionParts(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < max(len(pa), len(pb)); i++ {
		x, y := part(pa, i), part(pb, i)
		if x != y {
			if x < y {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

func versionParts(v string) ([]int, error) {
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil || strings.Trim(p, "0123456789") != "" {
			return nil, fmt.Errorf("%q is not a dotted numeric version", v)
		}
		out = append(out, n)
	}
	return out, nil
}

func part(parts []int, i int) int {
	if i < len(parts) {
		return parts[i]
	}
	return 0
}
