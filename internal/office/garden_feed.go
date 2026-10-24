package office

import (
	"regexp"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/layout"
	"github.com/dhitalkamal/clalt/internal/office/layout/garden"
)

// reviewType mirrors the page's routing: these subagent types sit on the
// treehouse review side, not in the workspace.
var reviewType = regexp.MustCompile(`review|test|hunter|analy`)

// GardenFeed turns snapshots into desk needs for the garden generators and
// serves the generated layouts. it is the Generator of the server.
type GardenFeed struct {
	tracker *garden.Tracker
	now     func() time.Time
}

// NewGardenFeed starts with no accounts. now is the clock the holds use.
func NewGardenFeed(now func() time.Time) *GardenFeed {
	if now == nil {
		now = time.Now
	}
	return &GardenFeed{tracker: garden.NewTracker(), now: now}
}

// Observe reads the needs off one snapshot: a cabin per account sized by its
// live main sessions, the workspace by the subagents that are not Explore,
// Plan or review types, research by the Explore ones.
func (g *GardenFeed) Observe(s Snapshot) {
	var accounts []garden.Account
	for _, a := range s.Accounts {
		if len(accounts) == garden.MaxAccounts {
			break
		}
		accounts = append(accounts, garden.Account{ID: a.ID, Label: a.Label})
	}
	needs := garden.Needs{CabinDesks: map[string]int{}}
	for _, sess := range s.Sessions {
		needs.CabinDesks[sess.Account]++
		for _, sub := range sess.Subagents {
			if sub.State == "done" || sub.EndedAt != nil {
				continue
			}
			switch t := strings.ToLower(sub.Type); {
			case t == "explore":
				needs.ResearchDesks++
			case t == "plan" || reviewType.MatchString(t):
			default:
				needs.WorkspaceDesks++
			}
		}
	}
	g.tracker.Observe(g.now(), accounts, needs)
}

// Layout implements Generator.
func (g *GardenFeed) Layout(template string) (layout.Layout, string, error) {
	return g.tracker.Layout(template)
}
