package summary

import (
	"testing"
	"time"
)

// two sessions on 2026-10-27 local time (utc-7): work does tool calls, a
// subagent run and a failure, personal does one edit.
func dayFixture(t *testing.T) (*Service, string) {
	root := t.TempDir()
	mt := at("2026-10-27T20:00:00Z")
	write(t, root, "-home-u-alpha", "s1", mt,
		asst("2026-10-27T16:00:00Z", "s1", "/home/u/alpha", "m1", 100, 50,
			toolUse("t1", "Edit", `{"file_path":"/home/u/alpha/src/a.go"}`),
			toolUse("t2", "Edit", `{"file_path":"/home/u/alpha/src/a.go"}`),
			toolUse("t3", "Read", `{"file_path":"/home/u/alpha/README.md"}`)),
		asst("2026-10-27T16:01:00Z", "s1", "/home/u/alpha", "m2", 200, 100,
			toolUse("t4", "Bash", `{"command":"make test"}`),
			toolUse("t5", "Agent", `{"subagent_type":"reviewer","description":"look at it"}`),
			toolUse("t6", "Agent", `{"subagent_type":"reviewer","description":"again"}`),
			toolUse("t7", "Task", `{"subagent_type":"explorer","description":"map"}`)),
		userLine("2026-10-27T16:02:00Z", "s1", "/home/u/alpha", toolResult("t4", true, "boom: build failed\nsecond line")),
		userLine("2026-10-27T16:03:00Z", "s1", "/home/u/alpha", toolResult("t1", false, "ok")),
		// a 20 minute gap is idle, the last two minutes are active
		asst("2026-10-27T16:23:00Z", "s1", "/home/u/alpha", "m3", 10, 10),
		asst("2026-10-27T16:25:00Z", "s1", "/home/u/alpha", "m4", 10, 10),
	)
	write(t, root, "-home-u-beta", "s2", mt,
		asst("2026-10-27T18:00:00Z", "s2", "/home/u/beta", "m5", 1000, 500,
			toolUse("u1", "Write", `{"file_path":"/home/u/beta/notes.txt"}`)),
	)
	return newService(root, "2026-10-27T21:00:00Z", map[string]string{"s1": "work", "s2": "personal"}), root
}

func TestDayPerAccountAndProject(t *testing.T) {
	s, _ := dayFixture(t)
	got, err := s.Day("2026-10-27")
	if err != nil {
		t.Fatal(err)
	}
	if got.Day != "2026-10-27" || got.Truncated {
		t.Fatalf("header: %+v", got)
	}
	if len(got.Accounts) != 2 || got.Accounts[0].ID != "personal" || got.Accounts[1].ID != "work" {
		t.Fatalf("accounts: %+v", got.Accounts)
	}
	w := got.Accounts[1]
	if w.Label != "WORK" || w.Sessions != 1 || w.Tokens != 100+50+200+100+10+10+10+10 {
		t.Fatalf("work: %+v", w.Stats)
	}
	if len(w.Projects) != 1 || w.Projects[0].Name != "alpha" {
		t.Fatalf("projects: %+v", w.Projects)
	}
	p := w.Projects[0]
	if p.Cost <= 0 {
		t.Errorf("cost: %v", p.Cost)
	}
	wantSubs := []Count{{"reviewer", 2}, {"explorer", 1}}
	if len(p.Subagents) != 2 || p.Subagents[0] != wantSubs[0] || p.Subagents[1] != wantSubs[1] {
		t.Errorf("subagents: %+v", p.Subagents)
	}
	wantTools := []Count{{"Edit", 2}, {"Bash", 1}, {"Read", 1}}
	if len(p.Tools) != 3 || p.Tools[0] != wantTools[0] || p.Tools[1] != wantTools[1] || p.Tools[2] != wantTools[2] {
		t.Errorf("tools: %+v", p.Tools)
	}
	if p.ToolCalls != 4 {
		t.Errorf("toolCalls: %d", p.ToolCalls)
	}
	if len(p.Files) != 2 || p.Files[0] != (Count{"src/a.go", 2}) || p.Files[1] != (Count{"README.md", 1}) {
		t.Errorf("files: %+v", p.Files)
	}
	if p.FailureCount != 1 || len(p.Failures) != 1 || p.Failures[0].Tool != "Bash" || p.Failures[0].Text != "boom: build failed" {
		t.Errorf("failures: %+v", p.Failures)
	}
	// gaps of 1, 1, 1 minute and 2 minutes count, the 20 minute one does not
	if p.ActiveMinutes != 5 {
		t.Errorf("active: %d", p.ActiveMinutes)
	}
	// the account rollup matches its only project
	if w.ToolCalls != p.ToolCalls || w.ActiveMinutes != p.ActiveMinutes || len(w.Subagents) != 2 {
		t.Errorf("rollup: %+v", w.Stats)
	}
	if b := got.Accounts[0]; b.Sessions != 1 || b.Projects[0].Name != "beta" || b.Tokens != 1500 {
		t.Errorf("personal: %+v", b)
	}
	if got.Totals.Sessions != 2 || got.Totals.Tokens != w.Tokens+1500 {
		t.Errorf("totals: %+v", got.Totals)
	}
}

func TestDayBoundariesAreLocal(t *testing.T) {
	root := t.TempDir()
	mt := at("2026-10-29T00:00:00Z")
	write(t, root, "-p", "s1", mt,
		// 2026-10-27 23:30 local (06:30Z next day): in the 27th
		asst("2026-10-28T06:30:00Z", "s1", "/p", "a", 10, 0),
		// 2026-10-28 00:10 local: in the 28th
		asst("2026-10-28T07:10:00Z", "s1", "/p", "b", 20, 0),
		// 2026-10-26 23:59 local: in the 26th
		asst("2026-10-27T06:59:00Z", "s1", "/p", "c", 40, 0),
	)
	s := newService(root, "2026-10-29T12:00:00Z", map[string]string{"s1": "work"})
	for day, want := range map[string]int64{"2026-10-26": 40, "2026-10-27": 10, "2026-10-28": 20} {
		got, err := s.Day(day)
		if err != nil {
			t.Fatal(err)
		}
		if got.Totals.Tokens != want {
			t.Errorf("%s: tokens %d, want %d", day, got.Totals.Tokens, want)
		}
	}
}

func TestDayDefaultsToToday(t *testing.T) {
	s, _ := dayFixture(t)
	// 21:00Z is 14:00 local on the 27th
	got, err := s.Day("")
	if err != nil || got.Day != "2026-10-27" {
		t.Fatalf("%v %+v", err, got.Day)
	}
}

func TestDayRejectsBadDates(t *testing.T) {
	s, _ := dayFixture(t)
	for _, d := range []string{"2026-13-01", "yesterday", "2026-10-27x", "26-10-27", "2026-10-27T00:00:00Z"} {
		if _, err := s.Day(d); err != ErrBadDay {
			t.Errorf("%q: %v", d, err)
		}
	}
}

func TestDayIgnoresFilesOlderThanTheDay(t *testing.T) {
	root := t.TempDir()
	// the mtime is before the day began, so nothing in it can belong to the day
	write(t, root, "-p", "old", at("2026-10-20T00:00:00Z"), asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 10, 0))
	s := newService(root, "2026-10-27T21:00:00Z", map[string]string{"s1": "work"})
	got, _ := s.Day("2026-10-27")
	if got.Totals.Tokens != 0 || len(got.Accounts) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestDaySkipsMalformedAndDuplicateLines(t *testing.T) {
	root := t.TempDir()
	l := asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 10, 5, toolUse("x1", "Read", `{"file_path":"/p/f"}`))
	write(t, root, "-p", "s1", at("2026-10-27T20:00:00Z"), "not json", l, l, `{"type":"assistant"`, "")
	s := newService(root, "2026-10-27T21:00:00Z", map[string]string{"s1": "work"})
	got, _ := s.Day("2026-10-27")
	if got.Totals.Tokens != 15 || got.Accounts[0].ToolCalls != 1 {
		t.Errorf("%+v", got.Accounts)
	}
}

func TestDayTopFilesCappedAt20(t *testing.T) {
	root := t.TempDir()
	var blocks []string
	for i := 0; i < 30; i++ {
		blocks = append(blocks, toolUse("t"+string(rune('a'+i%26))+string(rune('a'+i/26)), "Read", `{"file_path":"/p/f`+string(rune('a'+i%26))+string(rune('a'+i/26))+`"}`))
	}
	write(t, root, "-p", "s1", at("2026-10-27T20:00:00Z"), asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 1, 1, blocks...))
	s := newService(root, "2026-10-27T21:00:00Z", map[string]string{"s1": "work"})
	got, _ := s.Day("2026-10-27")
	if n := len(got.Accounts[0].Projects[0].Files); n != 20 {
		t.Errorf("files: %d", n)
	}
}

func TestDayCapsFilesAndBytes(t *testing.T) {
	root := t.TempDir()
	for i, name := range []string{"a", "b", "c"} {
		write(t, root, "-p", name, at("2026-10-27T10:00:00Z").Add(time.Duration(i)*time.Hour),
			asst("2026-10-27T16:00:00Z", name, "/p", name, 10, 0))
	}
	m := map[string]string{"a": "work", "b": "work", "c": "work"}
	s := New(Config{Roots: []string{root}, Accounts: accounts, Attribute: bySession(m), MaxFiles: 2,
		Now: func() time.Time { return at("2026-10-27T21:00:00Z") }, Loc: mst})
	got, _ := s.Day("2026-10-27")
	// the two newest files are read, the oldest is dropped
	if !got.Truncated || got.Totals.Tokens != 20 {
		t.Errorf("files cap: %+v tokens %d", got.Truncated, got.Totals.Tokens)
	}
	s = New(Config{Roots: []string{root}, Accounts: accounts, Attribute: bySession(m), MaxBytes: 1,
		Now: func() time.Time { return at("2026-10-27T21:00:00Z") }, Loc: mst})
	got, _ = s.Day("2026-10-27")
	if !got.Truncated || got.Totals.Tokens != 0 {
		t.Errorf("bytes cap: %+v tokens %d", got.Truncated, got.Totals.Tokens)
	}
}

func TestDayUnattributedAndStrings(t *testing.T) {
	root := t.TempDir()
	write(t, root, "-p", "s9", at("2026-10-27T20:00:00Z"),
		asst("2026-10-27T16:00:00Z", "s9", "/p/q\x1b[31m", "a", 5, 5, toolUse("z", "Read", `{"file_path":"/p/q/f\u0007.txt"}`)))
	s := newService(root, "2026-10-27T21:00:00Z", nil)
	got, _ := s.Day("2026-10-27")
	a := got.Accounts[0]
	if a.ID != "unattributed" || a.Label != "unattributed" {
		t.Errorf("%+v", a)
	}
	for _, c := range a.Projects[0].Files {
		for _, r := range c.Name {
			if r < 32 {
				t.Errorf("control char in %q", c.Name)
			}
		}
	}
}

func TestLabelsKeepEmailsOut(t *testing.T) {
	root := t.TempDir()
	write(t, root, "-p", "s1", at("2026-10-27T20:00:00Z"), asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 1, 1))
	s := New(Config{Roots: []string{root}, Attribute: bySession(map[string]string{"s1": "w"}),
		Accounts: func() ([]Account, error) { return []Account{{ID: "w", Label: "robin@example.com"}}, nil },
		Now:      func() time.Time { return at("2026-10-27T21:00:00Z") }, Loc: mst})
	got, _ := s.Day("2026-10-27")
	if got.Accounts[0].Label != "robin" {
		t.Errorf("label %q", got.Accounts[0].Label)
	}
}
