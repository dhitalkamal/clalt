package cli

import (
	"os"
	"path/filepath"
	"time"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/reviews"
)

// reviewCandidates turns the accounts into what the review picker wants: the
// folder claude is pointed at and the 5h percent, nil when that window is
// unknown or already over. an account with no folder is left out.
func reviewCandidates(accts []office.Account, lim office.LimitSource, now time.Time, dirFor func(id string) (string, bool)) []reviews.Candidate {
	var out []reviews.Candidate
	for _, a := range accts {
		dir, ok := dirFor(a.ID)
		if !ok {
			continue
		}
		c := reviews.Candidate{ID: a.ID, ConfigDir: dir}
		if l, ok := lim.Limits(a.ID); ok && l.FiveHour != nil && l.FiveHour.ResetsAt.After(now) {
			p := l.FiveHour.UsedPct
			c.FiveHourUsedPct = &p
		}
		out = append(out, c)
	}
	return out
}

// officeReviews is the PR review service of this machine: results and config
// under clalt's folder, clones and worktrees under ~/.cache/clalt/reviews.
func (a *App) officeReviews(real office.Real, logf func(string, ...any)) *reviews.Service {
	cache := filepath.Join(config.Home(), ".cache", "clalt", "reviews")
	bin, err := claudecli.Binary()
	if err != nil {
		// keep an explicit CLALT_CLAUDE_BIN so a configured path is never replaced
		bin = os.Getenv("CLALT_CLAUDE_BIN")
		if bin == "" {
			bin = "claude"
		}
	}
	x := reviews.RealExec{}
	res := reviews.DefaultResolver()
	return &reviews.Service{
		Dir: filepath.Join(config.ClaltDir(), "reviews"), CacheDir: cache, WorkDir: filepath.Join(cache, "work"),
		GH: reviews.GH{X: x, R: &res}, Git: reviews.Git{X: x}, Claude: x, ClaudeBin: bin, R: &res,
		EnvFor: func(dir string) []string { env, _ := claudecli.Env(os.Environ(), dir); return env },
		Accounts: func() []reviews.Candidate {
			accts, err := real.Accounts.Accounts()
			if err != nil {
				return nil
			}
			dirFor := func(id string) (string, bool) {
				dir, err := config.AccountDirFor(id)
				if err != nil {
					return "", false
				}
				fi, err := os.Stat(dir)
				return dir, err == nil && fi.IsDir()
			}
			return reviewCandidates(accts, real, a.clock(), dirFor)
		},
		Now: a.clock, Logf: logf,
	}
}
