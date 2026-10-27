package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/rules"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

const moveUsage = "clalt move <session> <account>"

// the session registry and the feed, as the wrapper sees them. variables so that
// a test can put fakes in their place.
var (
	openRegistry = func() (wrapper.Registry, error) { return sessionRegistry(), nil }
	openFeed     = func() wrapper.Feed { return realFeed() }
	// moveSupported is false where claude cannot be asked to end cleanly.
	moveSupported = osCanMove
)

// cmdMove asks a running `clalt run` session to move to another account. nothing
// moves here: a request is left next to the session's record, and the wrapper
// of the session makes the move at its next pause, see package wrapper. every
// check that can be made from here is made first, so a refusal is heard at once
// and not when the session is next idle.
func (a *App) cmdMove(args []string) error {
	ref, label, err := parseMoveArgs(args)
	if err != nil {
		return err
	}
	out, err := a.requestMove(ref, label, false)
	if err != nil {
		return err
	}
	if out.note != "" {
		fmt.Fprintf(a.Err, "note: %s\n", out.note)
	}
	if a.JSON {
		return a.emitJSON(map[string]any{
			"handle": out.rec.Handle, "from": out.from, "to": out.to, "requested": true, "replaced": out.replaced,
		})
	}
	a.printf("asked session %s (%s, %s) to move to %s at its next pause\n", short8(out.rec.Handle), out.from, clean(out.rec.Cwd), out.to)
	if out.replaced {
		a.printf("replaced the earlier request for this session\n")
	}
	return nil
}

// moveOutcome is a request that was left for a wrapper.
type moveOutcome struct {
	rec      wrapper.Record
	from, to string
	replaced bool
	// note is why the earlier request of the session was not carried out
	note string
}

// requestMove makes every check that can be made from here and leaves the
// request. exact says ref must be the whole handle, which the office wants: it
// names a session it already knows. the office page uses this too.
func (a *App) requestMove(ref, label string, exact bool) (moveOutcome, error) {
	if !moveSupported {
		return moveOutcome{}, errors.New("moving a session is not supported on windows: claude cannot be asked to end cleanly there")
	}
	reg, err := openRegistry()
	if err != nil {
		return moveOutcome{}, err
	}
	recs, err := reg.List()
	if err != nil {
		return moveOutcome{}, err
	}
	rec, err := matchRecord(recs, ref, exact)
	if err != nil {
		return moveOutcome{}, err
	}
	acc, err := a.findVisible(label)
	if err != nil {
		return moveOutcome{}, err
	}
	labels, err := a.labelsByID()
	if err != nil {
		return moveOutcome{}, err
	}
	from := labels[rec.Account]
	if from == "" {
		from = clean(rec.Account)
	}
	if acc.Name == rec.Account {
		return moveOutcome{}, fmt.Errorf("the session %s is already running as %s", short8(rec.Handle), from)
	}
	if rec.ClaudeSessionID == "" {
		return moveOutcome{}, errors.New("claude has not reported the session id of this session yet, so there is nothing to resume: try again in a moment")
	}
	info, err := moveFeedState(rec)
	if err != nil {
		return moveOutcome{}, err
	}
	r, err := a.Store.RulesOf(acc.Name)
	if err != nil {
		return moveOutcome{}, err
	}
	for _, cwd := range uniqueNonEmpty(rec.Cwd, info.Cwd) {
		if !rules.Allows(r.Folders, cwd) {
			return moveOutcome{}, fmt.Errorf("the folder rules of %s do not include %s, so the session cannot move there. 'clalt rules %s' shows them",
				shown(acc), clean(cwd), shown(acc))
		}
	}
	if why := a.notReady(acc); why != "" {
		return moveOutcome{}, fmt.Errorf("%s is not ready to take a session: %s", shown(acc), why)
	}

	out := moveOutcome{rec: rec, from: from, to: shown(acc)}
	if res, ok := wrapper.TakeResult(reg.Dir(), rec.Handle); ok {
		out.note = "the earlier request for this session was not carried out: " + clean(res.Error)
	}
	out.replaced = wrapper.HasRequest(reg.Dir(), rec.Handle)
	if err := wrapper.WriteRequest(reg.Dir(), rec.Handle, wrapper.Request{To: acc.Name, RequestedAt: time.Now()}); err != nil {
		return moveOutcome{}, err
	}
	return out, nil
}

// matchRecord is wrapper.Match, or with exact only the record of that handle.
func matchRecord(recs []wrapper.Record, ref string, exact bool) (wrapper.Record, error) {
	if !exact {
		return wrapper.Match(recs, ref)
	}
	for _, r := range recs {
		if r.Handle == ref {
			return r, nil
		}
	}
	return wrapper.Record{}, fmt.Errorf("no running session %s", clean(ref))
}

func parseMoveArgs(args []string) (ref, label string, err error) {
	var pos []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return "", "", usageError{"unknown flag " + arg, moveUsage}
		}
		pos = append(pos, arg)
	}
	switch {
	case len(pos) < 2:
		return "", "", usageError{"which session, and which account? give both", moveUsage}
	case len(pos) > 2:
		return "", "", usageError{"unexpected argument " + pos[2], moveUsage}
	}
	return pos[0], pos[1], nil
}

// moveFeedState is what the feed says about the session. without it its pause
// could not be seen, and a request would wait for ever.
func moveFeedState(rec wrapper.Record) (wrapper.SessionInfo, error) {
	feed := openFeed()
	if feed == nil {
		return wrapper.SessionInfo{}, errors.New("there is no feed state for this session, so its pause cannot be seen")
	}
	info, ok := feed.Session(rec.Handle)
	switch {
	case !ok:
		return info, errors.New("the feed has no state for this session, so its pause cannot be seen: are claude's hooks installed?")
	case info.State == wrapper.Ended:
		return info, errors.New("the session has ended")
	}
	return info, nil
}

func short8(handle string) string {
	if len(handle) > 8 {
		return handle[:8]
	}
	return handle
}

func uniqueNonEmpty(items ...string) []string {
	var out []string
	for _, s := range items {
		if s == "" {
			continue
		}
		dup := false
		for _, o := range out {
			dup = dup || o == s
		}
		if !dup {
			out = append(out, s)
		}
	}
	return out
}
