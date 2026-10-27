// Package activity reads the activity log of one claude session out of its
// transcript on demand: prompts, replies, tool calls with their results and
// durations, and subagent runs. it only reads, keeps nothing, and cleans and
// caps everything it returns, since a transcript is not trusted text.
package activity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
)

// defaults and bounds, see Options.
const (
	defaultLimit  = 100
	maxLimit      = 300
	maxScan       = 64 << 20 // bytes read per request
	maxLine       = 16 << 20 // longest line read, as in internal/usage
	maxReply      = 2 << 20  // bytes of entries in one reply
	maxExtraAfter = 50       // entries past the limit to keep a call with its result
	maxBehind     = 200      // lines below the After bound read to pair a late result
	entryOverhead = 160      // bytes an entry costs besides its text, for the reply cap
	cursorPrefix  = "v1:"
)

// ErrBadCursor is returned for a cursor that this package did not make for
// this file.
var ErrBadCursor = errors.New("activity: bad cursor")

// Source says what to read. Path is a transcript file. Agent is the subagent
// whose run is wanted, empty for the main conversation. Legacy says that Path
// is the main file and the run is the sidechain lines in it with that agent id
// (older claude wrote them there), where otherwise the file is the run itself.
type Source struct {
	Path   string
	Agent  string
	Legacy bool
}

// Options is one request. zero values mean the defaults: Limit 100 (at most
// 300), MaxScan 64 MB, MaxLine 16 MB. Before is a cursor from a reply.
type Options struct {
	Source
	Before string
	// After is a cursor from a reply's Newest: only entries newer than it are
	// read. with Before too, the entries between the two.
	After   string
	Limit   int
	MaxScan int64
	MaxLine int
}

// Entry is one thing that happened, see docs/plans/office-work.md.
type Entry struct {
	T          string   `json:"t"`
	Kind       string   `json:"kind"`
	Text       string   `json:"text"`
	Tool       string   `json:"tool"`
	Files      []string `json:"files"`
	DurationMs int64    `json:"durationMs"`
	OK         bool     `json:"ok"`
	Agent      string   `json:"agent"`

	drop bool
}

// Reply is a page of entries, newest first. Next is the cursor for older ones,
// empty when there are none. Skipped counts lines that could not be read.
type Reply struct {
	Handle string `json:"handle"`
	Agent  string `json:"agent"`
	Next   string `json:"next"`
	// Newest is the cursor to pass as After to get what comes after this
	// reply. it is set on the newest page only, not on one reached by Before.
	Newest  string  `json:"newest"`
	Entries []Entry `json:"entries"`
	Skipped int     `json:"skipped"`
}

func encodeCursor(off int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(cursorPrefix + strconv.FormatInt(off, 10)))
}

func decodeCursor(c string, size int64) (int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return 0, ErrBadCursor
	}
	num, ok := strings.CutPrefix(string(raw), cursorPrefix)
	off, err := strconv.ParseInt(num, 10, 64)
	if !ok || err != nil || off < 0 || off > size {
		return 0, ErrBadCursor
	}
	return off, nil
}

// reader is the state of one Read.
type reader struct {
	o       Options
	out     []Entry
	pend    map[string]*resultInfo
	bytes   int
	skipped int
	// quiet is set below the After bound: results get their call, nothing is added
	quiet bool
}

// Read returns a page of the activity of o.Path, newest first, reading from
// the end of the file (or from the cursor) and at most MaxScan bytes. a file
// that is not there is an error that os.IsNotExist says yes to.
func Read(o Options) (Reply, error) {
	if o.Limit <= 0 {
		o.Limit = defaultLimit
	}
	if o.Limit > maxLimit {
		o.Limit = maxLimit
	}
	if o.MaxScan <= 0 {
		o.MaxScan = maxScan
	}
	if o.MaxLine <= 0 {
		o.MaxLine = maxLine
	}
	f, err := os.Open(o.Path)
	if err != nil {
		return Reply{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return Reply{}, err
	}
	end := fi.Size()
	if o.Before != "" {
		if end, err = decodeCursor(o.Before, fi.Size()); err != nil {
			return Reply{}, err
		}
	}
	lower := int64(-1)
	if o.After != "" {
		if lower, err = decodeCursor(o.After, fi.Size()); err != nil {
			return Reply{}, err
		}
	}
	rd := &reader{o: o, pend: map[string]*resultInfo{}}
	br := newBackReader(f, end, o.MaxScan, o.MaxLine)
	last, newest, extra, behind := int64(-1), int64(-1), 0, 0
	crossed := false
	for {
		line, start, ok := br.next()
		if !ok {
			break
		}
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		if start <= lower {
			// everything newer is read. older lines are only looked at to name
			// the call of a result that came in after the last poll
			crossed, rd.quiet = true, true
			if behind++; len(rd.pend) == 0 || behind > maxBehind {
				break
			}
			rd.line(line)
			continue
		}
		last = start
		if rd.line(line) && newest < 0 {
			newest = start
		}
		if len(rd.out) >= o.Limit || rd.bytes >= maxReply {
			// keep a tool call on the page of its result, a little over the limit at most
			if rd.bytes >= maxReply || len(rd.pend) == 0 || extra >= maxExtraAfter {
				break
			}
			extra++
		}
	}
	r := Reply{Agent: o.Agent, Entries: []Entry{}, Skipped: rd.skipped + br.skipped}
	for _, e := range rd.out {
		if !e.drop {
			r.Entries = append(r.Entries, e)
		}
	}
	if last > 0 && !crossed {
		r.Next = encodeCursor(last)
	}
	if o.Before == "" {
		switch {
		case newest >= 0:
			r.Newest = encodeCursor(newest)
		case o.After != "":
			r.Newest = o.After
		}
	}
	return r, nil
}

// wanted says whether a line belongs to the view asked for.
func (rd *reader) wanted(l rawLine) bool {
	switch {
	case rd.o.Agent == "":
		return !l.IsSidechain
	case rd.o.Legacy:
		return l.IsSidechain && l.AgentID == rd.o.Agent
	}
	return true
}

// line reads one line and adds its entries, newest first. ok is false for a
// line that is not json.
func (rd *reader) line(raw []byte) (ok bool) {
	var l rawLine
	if err := json.Unmarshal(raw, &l); err != nil {
		if !rd.quiet {
			rd.skipped++
		}
		return false
	}
	if !rd.wanted(l) {
		return true
	}
	ps := pieces(l, parseTime(l.Timestamp))
	for i := len(ps) - 1; i >= 0; i-- {
		rd.add(ps[i])
	}
	return true
}

// add appends one entry. going backwards a result is met before its call, so
// results wait in pend and the call completes them.
func (rd *reader) add(p piece) {
	if rd.quiet && (p.useID == "" || rd.pend[p.useID] == nil) {
		return
	}
	e := p.Entry
	e.Agent = rd.o.Agent
	if p.useID != "" {
		if info := rd.pend[p.useID]; info != nil {
			delete(rd.pend, p.useID)
			rd.pair(&e, p, info)
		}
	}
	if rd.quiet {
		return
	}
	if p.res != nil {
		p.res.idx = len(rd.out)
		rd.pend[p.resID] = p.res
	}
	rd.bytes += entryOverhead + len(e.Text) + len(e.Tool)
	for _, f := range e.Files {
		rd.bytes += len(f)
	}
	rd.out = append(rd.out, e)
}

// pair gives the call and its result the tool name, the duration and the
// outcome. a subagent call and its result become the start and the end of a
// run. the result of a run that went to the background only says it started,
// so it is dropped.
func (rd *reader) pair(call *Entry, p piece, info *resultInfo) {
	res := &rd.out[info.idx]
	if d := info.at.Sub(parseTime(call.T)).Milliseconds(); d > 0 && !parseTime(call.T).IsZero() {
		call.DurationMs, res.DurationMs = d, d
	}
	call.OK = info.ok
	res.Tool = call.Tool
	if isAgentTool(p.useName) {
		call.Agent = info.agent
		res.Kind, res.Agent = KindSubagentEnd, info.agent
		if info.async {
			res.drop = true
			call.DurationMs, call.OK = 0, true
		}
	}
}
