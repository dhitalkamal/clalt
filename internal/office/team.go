package office

import (
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"sort"
	"strings"
)

// the team of the snapshot: every agent type with a desk, see the contract in
// docs/plans/team-office.md.
const (
	maxTeam     = 24
	maxTeamName = 40
	maxTeamDesc = 120
)

// TeamOut is one agent type of the team. Source is builtin, user, project or
// seen, Busy the running subagents of the type across all sessions.
type TeamOut struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Source string `json:"source"`
	Busy   int    `json:"busy"`
	Color  string `json:"color"`
}

var (
	teamIDRe = regexp.MustCompile(`^[a-z0-9:-]{1,64}$`)
	// reviewers seen in the feed sit right after the built-ins, the page's
	// own rule for the review side
	reviewerRe = regexp.MustCompile(`review|test|hunter|analy`)
)

// builtinTeam is what every claude has, in desk order.
var builtinTeam = []TeamOut{
	{ID: "general-purpose", Name: "general-purpose", Desc: "multi step work and research", Source: "builtin"},
	{ID: "explore", Name: "Explore", Desc: "fast read only codebase search", Source: "builtin"},
	{ID: "plan", Name: "Plan", Desc: "designs an implementation plan", Source: "builtin"},
}

// teamOut builds the team from the custom agent files and the subagents of
// the (already cleaned) sessions. the list is never nil.
func teamOut(defs []AgentDef, sessions []SessionOut) []TeamOut {
	byID := map[string]*TeamOut{}
	var order []*TeamOut
	add := func(m TeamOut) *TeamOut {
		p := &m
		byID[m.ID] = p
		order = append(order, p)
		return p
	}
	for _, b := range builtinTeam {
		add(b)
	}
	// project beats user, as claude resolves a clash; a built-in is never replaced
	for _, source := range []string{"user", "project"} {
		for _, d := range defs {
			if d.Source != source {
				continue
			}
			name := noEmail(clean(d.Name, maxTeamName))
			id := strings.ToLower(name)
			if !teamIDRe.MatchString(id) {
				continue
			}
			m := TeamOut{ID: id, Name: name, Desc: teamDesc(d.Desc), Source: source}
			if old := byID[id]; old != nil {
				if old.Source == "builtin" {
					continue
				}
				*old = m
				continue
			}
			add(m)
		}
	}
	for _, s := range sessions {
		for _, sub := range s.Subagents {
			name := clean(sub.Type, maxTeamName)
			id := strings.ToLower(name)
			if !teamIDRe.MatchString(id) {
				continue
			}
			m := byID[id]
			if m == nil {
				m = add(TeamOut{ID: id, Name: name, Source: "seen"})
			}
			if sub.State != "done" && sub.EndedAt == nil {
				m.Busy++
			}
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if ra, rb := teamRank(a), teamRank(b); ra != rb {
			return ra < rb
		}
		if a.Source == "builtin" {
			return false
		}
		if an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name); an != bn {
			return an < bn
		}
		return a.ID < b.ID
	})
	if len(order) > maxTeam {
		order = order[:maxTeam]
	}
	out := make([]TeamOut, len(order))
	for i, m := range order {
		m.Color = teamColor(m.ID)
		out[i] = *m
	}
	return out
}

// teamRank is the group of a member: built-ins, reviewers that were only
// seen, then user, project and the other seen types.
func teamRank(m *TeamOut) int {
	switch {
	case m.Source == "builtin":
		return 0
	case m.Source == "seen" && reviewerRe.MatchString(m.ID):
		return 1
	case m.Source == "user":
		return 2
	case m.Source == "project":
		return 3
	}
	return 4
}

// teamDesc cleans a description. an at sign cuts it like an email in a name,
// but a description that starts with one is left empty.
func teamDesc(s string) string {
	s = clean(s, maxTeamDesc)
	if i := strings.Index(s, "@"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// teamColor is a colour from the hash of the id, the same every time.
func teamColor(id string) string {
	h := fnv.New32a()
	h.Write([]byte(id))
	r, g, b := hsl(float64(h.Sum32()%360), 0.58, 0.52)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// hsl converts hue in degrees, saturation and lightness (0 to 1) to 8 bit rgb.
func hsl(h, s, l float64) (r, g, b uint8) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var rf, gf, bf float64
	switch int(h / 60) {
	case 0:
		rf, gf, bf = c, x, 0
	case 1:
		rf, gf, bf = x, c, 0
	case 2:
		rf, gf, bf = 0, c, x
	case 3:
		rf, gf, bf = 0, x, c
	case 4:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}
	to := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return to(rf), to(gf), to(bf)
}
