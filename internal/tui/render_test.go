package tui

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderPanels(t *testing.T) {
	out := Render(sampleData(), 120, 40)
	for _, want := range []string{"clalt", "Accounts", "Sessions", "work", "personal"} {
		if !strings.Contains(out, want) {
			t.Errorf("frame missing %q", want)
		}
	}
	for _, gone := range []string{"Keepalive", "keepalive", "expire", "resets"} {
		if strings.Contains(out, gone) {
			t.Errorf("frame still mentions %q", gone)
		}
	}
	// usage "tokens" are fine, the login token is gone
	if regexp.MustCompile(`\btoken\b`).MatchString(out) {
		t.Error("frame still has a token row")
	}
}

func TestAccountsThatNeedALoginAreFlagged(t *testing.T) {
	m := dashModel{data: sampleData(), width: 120, height: 40, focus: focusAccounts}
	rows := map[string]string{}
	for _, line := range strings.Split(m.renderAccounts(30, 8), "\n") {
		for _, name := range []string{"work", "personal", "casey", "jordan"} {
			if strings.Contains(line, name) {
				rows[name] = line
			}
		}
	}
	// signed out (casey) and due within three days (personal) get a mark; the others do not
	for name, wantFlag := range map[string]bool{"work": false, "personal": true, "casey": true, "jordan": false} {
		row, ok := rows[name]
		if !ok {
			t.Fatalf("no row for %s", name)
		}
		if got := strings.Contains(row, "!"); got != wantFlag {
			t.Errorf("%s: flagged = %v, want %v: %q", name, got, wantFlag, row)
		}
	}
}

func TestDetailShowsTheSignInState(t *testing.T) {
	d := sampleData()
	work := Render(d, 120, 40)
	if !strings.Contains(work, "signed in") || !strings.Contains(work, "sign in again by "+d.Accounts[0].LoginDue.Local().Format("2006-01-02")) {
		t.Errorf("a signed in account shows when to sign in again:\n%s", work)
	}
	m := dashModel{data: d, sel: 2, width: 120, height: 40, focus: focusAccounts} // casey is signed out
	if out := m.View(); !strings.Contains(out, "signed out") || strings.Contains(out, "sign in again by") {
		t.Errorf("a signed out account says so:\n%s", out)
	}
}

func TestRenderDetailContent(t *testing.T) {
	out := Render(sampleData(), 120, 40)
	// detail pane for the selected account (work): windows, today, budget, email
	for _, want := range []string{"5h", "7d", "today", "budget", "work@acme.com", "opus"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail pane missing %q", want)
		}
	}
}

func TestTooSmall(t *testing.T) {
	out := Render(sampleData(), 70, 20)
	if !strings.Contains(out, "too small") {
		t.Errorf("small terminal should warn, got:\n%s", out)
	}
}

func TestFocusCycle(t *testing.T) {
	m := newTestModel(&fakeController{})
	m = pump(m, "tab")
	if m.focus != focusSessions {
		t.Fatalf("tab -> focus %d, want sessions", m.focus)
	}
	m = pump(m, "tab")
	if m.focus != focusAccounts {
		t.Fatalf("tab*2 should wrap to accounts, got %d", m.focus)
	}
	m = pump(m, "shift+tab")
	if m.focus != focusSessions {
		t.Fatalf("shift+tab from accounts -> %d, want sessions", m.focus)
	}
}

func TestSelectionMovesDetail(t *testing.T) {
	m := newTestModel(&fakeController{})
	m = pump(m, "down")
	if m.sel != 1 {
		t.Fatalf("down -> sel %d, want 1", m.sel)
	}
	if !strings.Contains(m.View(), "personal") {
		t.Error("detail should follow selection to personal")
	}
}

func TestRunAction(t *testing.T) {
	m := newTestModel(&fakeController{})
	nm, _ := m.Update(key("enter"))
	m = nm.(dashModel)
	if m.action.Kind != "run" || m.action.Account != "work" {
		t.Errorf("enter -> %+v, want run work", m.action)
	}
}

func TestRenameModal(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m = pump(m, "R")
	if m.modal.kind != "rename" {
		t.Fatalf("R should open rename modal, got %q", m.modal.kind)
	}
	m = pump(m, "n", "e", "w", "enter")
	if f.renamed != [2]string{"work", "new"} {
		t.Errorf("rename recorded %v, want [work new]", f.renamed)
	}
}

func TestBudgetModal(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m = pump(m, "e")
	if m.modal.kind != "budget" {
		t.Fatalf("e should open budget modal, got %q", m.modal.kind)
	}
	m = pump(m, "1", "0", "enter", "2", "0", "enter")
	if f.budget != [3]string{"work", "10", "20"} {
		t.Errorf("budget recorded %v, want [work 10 20]", f.budget)
	}
}

func TestDeleteConfirm(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m = pump(m, "down") // personal, not active
	m = pump(m, "d")
	if !m.confirm.active {
		t.Fatal("d should open a confirm")
	}
	m = pump(m, "y")
	if f.removed != "personal" {
		t.Errorf("remove recorded %q, want personal", f.removed)
	}
}

func TestLogoutConfirm(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m = pump(m, "l", "y")
	if f.loggedOut != "work" {
		t.Errorf("logout recorded %q, want work", f.loggedOut)
	}
}

func TestSessionKill(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m = pump(m, "tab") // focus sessions
	m = pump(m, "k", "y")
	if f.killed != 8123 {
		t.Errorf("kill recorded %d, want 8123", f.killed)
	}
}

func TestLoginModalReturnsAction(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m = pump(m, "n") // login prompts for a name
	if m.modal.kind != "login" {
		t.Fatalf("n should open login modal, got %q", m.modal.kind)
	}
	for _, r := range "new" {
		m = pump(m, string(r))
	}
	nm, _ := m.Update(key("enter"))
	m = nm.(dashModel)
	if m.action.Kind != "login" || m.action.Account != "new" {
		t.Errorf("login modal -> %+v, want login new", m.action)
	}
}

func TestPaletteExcludesRemovedVerbs(t *testing.T) {
	m := newTestModel(&fakeController{})
	m = pump(m, ":")
	cmds := m.paletteCommands()
	var joined string
	for _, c := range cmds {
		joined += c.label + "\n"
	}
	for _, want := range []string{"run", "login", "rename", "budget", "logout", "remove"} {
		if !strings.Contains(joined, want) {
			t.Errorf("palette missing verb %q", want)
		}
	}
	for _, banned := range []string{"switch", "rotate", "next", "auto", "fallback", "refresh", "add"} {
		if strings.Contains(joined, banned) {
			t.Errorf("palette must not list removed verb %q", banned)
		}
	}
}

func TestFilterMode(t *testing.T) {
	m := newTestModel(&fakeController{})
	m = pump(m, "/")
	if m.mode != "filter" {
		t.Fatalf("/ -> mode %q, want filter", m.mode)
	}
	m = pump(m, "p", "e", "r")
	vis := m.visibleAccounts()
	if len(vis) != 1 || vis[0].Name != "personal" {
		t.Errorf("filter 'per' -> %d accounts, want [personal]", len(vis))
	}
}

func TestTickRefreshesData(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	// simulate a data refresh landing: mutate the fake, deliver a dataMsg
	f.data.Accounts[0].Name = "renamed-work"
	nm, _ := m.Update(dataMsg{data: f.data})
	m = nm.(dashModel)
	if m.data.Accounts[0].Name != "renamed-work" {
		t.Error("dataMsg should replace the model data")
	}
}

func TestQuit(t *testing.T) {
	m := newTestModel(&fakeController{})
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Error("q should return a quit command")
	}
}
