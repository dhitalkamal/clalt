package reviews

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Finding is one thing the review found.
type Finding struct {
	Severity string `json:"severity"` // high, medium or low
	File     string `json:"file"`
	Line     int    `json:"line"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

// Review is the stored result of one run on one head commit. Skipped holds why
// nothing was reviewed (too big), Error why the run failed. Transcript is the
// claude session id of the run, so the activity reader can show its log.
type Review struct {
	Repo       string    `json:"repo"`
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	URL        string    `json:"url"`
	SHA        string    `json:"sha"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt"`
	Account    string    `json:"account"`
	Findings   []Finding `json:"findings"`
	Summary    string    `json:"summary"`
	Transcript string    `json:"transcript"`
	Skipped    string    `json:"skipped,omitempty"`
	Error      string    `json:"error,omitempty"`
	// Attempts counts failed runs, NextRetryAt is when a failed run is tried again.
	Attempts    int        `json:"attempts,omitempty"`
	NextRetryAt *time.Time `json:"nextRetryAt,omitempty"`
	// State is the pull request state when last checked, StateCheckedAt when.
	State          string     `json:"state,omitempty"`
	StateCheckedAt *time.Time `json:"stateCheckedAt,omitempty"`
}

// the states a review's pull request can be in. unknown is also what a review
// stored before state existed reads as.
const (
	StateOpen    = "open"
	StateMerged  = "merged"
	StateClosed  = "closed"
	StateUnknown = "unknown"
)

// idRe is what an id looks like: owner-repo-number-sha7.
var idRe = regexp.MustCompile(`^[A-Za-z0-9._-]+-[0-9]+-[0-9a-f]{7}$`)

// idOf is the id of a pull request at a commit.
func idOf(repo string, number int, sha string) string {
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return strings.ReplaceAll(repo, "/", "-") + "-" + strconv.Itoa(number) + "-" + sha
}

// ID names the review, and its file.
func (r Review) ID() string { return idOf(r.Repo, r.Number, r.SHA) }
