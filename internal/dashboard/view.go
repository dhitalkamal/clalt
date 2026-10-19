package dashboard

import "time"

type viewID int

const (
	viewOverview viewID = iota
	viewSessions
	viewLimits
	viewHistory
	viewProfile
)

var tabNames = []string{"Overview", "Sessions", "Limits", "History", "Profile"}

var subNames = []string{"Days", "Projects", "Models"}

// content is what a view puts under the tab row: fixed head lines, scrolling
// body lines and, for list views, where the cursor rows are. rows counts them,
// first is the body line of the first, stride the lines from one to the next
// and span the lines each takes.
type content struct {
	head   []rich
	body   []rich
	rows   int
	first  int
	stride int
	span   int
}

// viewCtx is what a view needs to know besides the account
type viewCtx struct {
	g       glyphs
	w       int // text width inside the pane
	h       int // rows under the tab row and its blank line
	focused bool
	row     int    // cursor row in list views
	sub     int    // history tab
	drill   string // project open in the drill-in, empty when the table shows
	now     time.Time
}
