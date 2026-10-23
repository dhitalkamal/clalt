package wrapper

import (
	"fmt"
	"time"

	"github.com/dhitalkamal/clalt/internal/pick"
)

// Config holds the numbers the wrapper works by. zero fields take their default.
type Config struct {
	Poll     time.Duration // how often the request file and the feed are looked at
	Settle   time.Duration // how long a session has to be idle before it counts as a pause
	Grace    time.Duration // how long claude gets to end after SIGTERM before it is killed
	KillWait time.Duration // how long a killed claude gets to be gone
	Cooldown time.Duration // no auto-switch this soon after a move, and no retry of a target that failed
	// Threshold is the used percentage of the 5-hour window at which auto-switch
	// moves a session. Margin is how many points more room a target must have.
	Threshold, Margin float64
}

func (c Config) withDefaults() Config {
	set := func(d *time.Duration, v time.Duration) {
		if *d == 0 {
			*d = v
		}
	}
	set(&c.Poll, 500*time.Millisecond)
	set(&c.Settle, 1500*time.Millisecond)
	set(&c.Grace, 15*time.Second)
	set(&c.KillWait, 5*time.Second)
	set(&c.Cooldown, 30*time.Minute)
	if c.Threshold == 0 {
		c.Threshold = 90
	}
	if c.Margin == 0 {
		c.Margin = 20
	}
	return c
}

// PolicyInput is what auto-switch looks at to decide about one idle session.
type PolicyInput struct {
	Now        time.Time
	Cfg        Config
	Current    pick.Candidate // the account the session is on
	Candidates []pick.Candidate
	Limits     func(id string) (pick.Limits, bool)
	LimitHit   bool // claude said the limit was reached
	Cwd        string
	LastMove   time.Time            // the last move of this session, zero if none
	Blocked    map[string]time.Time // accounts a move to failed for, and when
}

// Decision is a move auto-switch wants, and the reason to say.
type Decision struct {
	To  pick.Candidate
	Why string
}

// Decide says whether an idle session should leave its account, and for which.
// it does when auto-switch is on for its account, the 5-hour window has reached
// the threshold or claude reported the limit, the last move was long enough ago,
// and there is an allowed account, with auto-switch on, that is known to have
// clearly more room. nothing qualifying is no move, and nothing is said.
func Decide(in PolicyInput) (Decision, bool) {
	cfg := in.Cfg.withDefaults()
	if !in.Current.AutoSwitch {
		return Decision{}, false
	}
	if !in.LastMove.IsZero() && in.Now.Sub(in.LastMove) < cfg.Cooldown {
		return Decision{}, false
	}
	cur, haveCur := in.Limits(in.Current.ID)
	var why string
	switch used, known := cur.FiveHourUsed(in.Now); {
	case in.LimitHit:
		why = "claude reported the limit"
	case haveCur && known && cur.Recent(in.Now, pick.DefaultMaxAge) && used >= cfg.Threshold:
		why = fmt.Sprintf("5-hour window at %.0f%% (switches at %.0f%%)", used, cfg.Threshold)
	default:
		return Decision{}, false
	}
	room := 0.0
	if r, ok := cur.Room(in.Now); ok && !in.LimitHit && haveCur {
		room = r
	}

	var open []pick.Candidate
	for _, c := range in.Candidates {
		if at, failed := in.Blocked[c.ID]; failed && in.Now.Sub(at) < cfg.Cooldown {
			continue
		}
		open = append(open, c)
	}
	res := pick.Rank(open, pick.Options{
		Cwd: in.Cwd, Now: in.Now, Limits: in.Limits, Exclude: in.Current.ID, NeedAutoSwitch: true,
	})
	for _, r := range res.Ranked {
		if r.HasData && r.Room >= room+cfg.Margin {
			return Decision{To: r.Candidate, Why: why}, true
		}
	}
	return Decision{}, false
}
