package dashboard

import (
	"fmt"
	"time"
)

// readings older than this are probably stale, so the age turns yellow
const oldReadings = time.Hour

func overviewContent(c viewCtx, a Account) content {
	b := identityRows(a, c.w)
	if a.Problem != "" {
		b = append(b, hang(rich{bad("!")}, 2, rich{bad(a.Problem)}, c.w)...)
	}
	if !a.SignedIn {
		return content{body: append(b, signInPrompt())}
	}
	b = append(b, statusRows(c, a)...)
	b = append(b, nil)
	b = append(b, limitBlock(c, a)...)
	b = append(b, ageRow(c, a), nil)
	b = append(b, budgetRows(c, a)...)
	b = append(b, nil)
	b = append(b, usageRows(c, a)...)
	b = append(b, nil)
	b = append(b, runningRows(a, c.w)...)
	return content{body: b}
}

func identityRows(a Account, w int) []rich {
	email := rich{txt(a.Email)}
	if a.Email == "" {
		email = rich{dim("email unknown")}
	}
	var tail rich
	parts := []string{a.Org}
	if a.SignedIn {
		parts = append(parts, a.Plan)
	}
	for _, p := range parts {
		if p == "" {
			continue
		}
		if len(tail) > 0 {
			tail = append(tail, gap(3))
		}
		tail = append(tail, dim(p))
	}
	if len(tail) == 0 {
		return []rich{email}
	}
	if one := cat(email, rich{gap(3)}, tail); one.width() <= w {
		return []rich{one}
	}
	return []rich{email, tail}
}

func dueSentence(days int) string {
	if days <= 0 {
		return "Re-login is due now."
	}
	return fmt.Sprintf("Re-login due in %s.", plural(days, "day"))
}

// statusRows keeps the login and last used sentences on one line when they
// fit and gives each its own line when they do not
func statusRows(c viewCtx, a Account) []rich {
	login := rich{dim("Signed in.")}
	if !a.Login.DueAt.IsZero() {
		days := daysLeft(a.Login.DueAt, c.now)
		login = rich{dim("Signed in. "), span{lvTone(loginLevel(days), toneDim), dueSentence(days)}}
	}
	used := "Never used."
	if !a.LastUsedAt.IsZero() {
		used = "Last used " + agoText(c.now.Sub(a.LastUsedAt)) + "."
	}
	seen := rich{dim(used)}
	if one := cat(login, rich{gap(1)}, seen); one.width() <= c.w {
		return []rich{one}
	}
	return append(wrap(login, c.w), wrap(seen, c.w)...)
}

// ageRow turns yellow once the limit readings are an hour old
func ageRow(c viewCtx, a Account) rich {
	if a.Limits.ReadAt.IsZero() {
		return rich{warn("No limit reading yet.")}
	}
	age := c.now.Sub(a.Limits.ReadAt)
	t := toneDim
	if age >= oldReadings {
		t = toneWarn
	}
	return rich{span{t, "Limits as of " + agoText(age) + "."}}
}

// resetVariants run from the most to the least text. ref is the width the
// layout reserves for the text, so the bars line up with the approved frames
// and a longer reset text shrinks the bar instead of being cut.
var resetVariants = []struct {
	ref  int
	text func(x Window, weekly bool, now time.Time) string
}{
	{27, func(x Window, weekly bool, now time.Time) string {
		if x.ResetsAt.IsZero() {
			return ""
		}
		return "  resets " + resetClock(x, weekly, now) + " (" + countdownText(x.ResetsAt.Sub(now), weekly) + ")"
	}},
	{18, func(x Window, weekly bool, now time.Time) string {
		if x.ResetsAt.IsZero() {
			return ""
		}
		return "  resets " + resetClock(x, weekly, now)
	}},
	{0, func(Window, bool, time.Time) string { return "" }},
}

// limitBlock draws the 5h and 7d bars at one width
func limitBlock(c viewCtx, a Account) []rich {
	wins := []struct {
		label  string
		x      Window
		weekly bool
	}{{"5h", a.Limits.Five, false}, {"7d", a.Limits.Seven, true}}
	text := resetVariants[len(resetVariants)-1].text
	reserve := 0
	for _, v := range resetVariants {
		need := 0
		for _, p := range wins {
			need = max(need, rich{txt(v.text(p.x, p.weekly, c.now))}.width())
		}
		// label, two spaces, a space, the percent and two spare cells
		if c.w-(11+max(need, v.ref)) >= 8 {
			text, reserve = v.text, max(need, v.ref)
			break
		}
	}
	barW := clampInt(c.w-11-reserve, 6, 32)
	var out []rich
	for _, p := range wins {
		out = append(out, cat(rich{dim(p.label), gap(2)}, usageBar(c.g, float64(p.x.Percent), barW),
			rich{gap(1), span{pctTone(p.x.Percent), fmt.Sprintf("%3d%%", p.x.Percent)}, dim(text(p.x, p.weekly, c.now))}))
		if pace := paceShort(p.x, p.weekly, c.now); pace != "" {
			out = append(out, hang(rich{gap(4)}, 4, rich{span{lvTone(paceLevel(p.x, c.now), toneDim), pace}}, c.w)...)
		}
	}
	return out
}

func budgetRows(c viewCtx, a Account) []rich {
	type capRow struct {
		label        string
		spent, limit float64
	}
	var caps []capRow
	if a.Budget.Daily > 0 {
		caps = append(caps, capRow{"today", a.today().cost, a.Budget.Daily})
	}
	if a.Budget.Weekly > 0 {
		caps = append(caps, capRow{"week", a.week().cost, a.Budget.Weekly})
	}
	if len(caps) == 0 {
		return hang(rich{dim("Budget")}, 9, prompt("not set. Press ", "e", " to set one."), c.w)
	}
	sw, cw := 0, 0
	for _, p := range caps {
		sw, cw = max(sw, len(money(p.spent))), max(cw, len(money(p.limit)))
	}
	barW := clampInt(clampInt(c.w-38, 8, 32)-8, 6, 24)
	rows := make([]rich, len(caps))
	for i, p := range caps {
		label := ""
		if i == 0 {
			label = "Budget"
		}
		line := rich{dim(label), gap(9 - len(label)), dim(fmt.Sprintf("%-5s", p.label)), gap(1),
			txt(fmt.Sprintf("%*s", sw, money(p.spent))), dim(" of "), txt(fmt.Sprintf("%*s", cw, money(p.limit)))}
		withBar := cat(line, rich{gap(3)}, usageBar(c.g, p.spent/p.limit*100, barW))
		rows[i] = firstFit(c.w, withBar, line)
	}
	return rows
}

func usageRows(c viewCtx, a Account) []rich {
	tail := func(u usageTotal) rich {
		return rich{txt(tokensText(u.tokens)), dim(" tokens"), gap(3), txt(approx(u.cost))}
	}
	today := cat(rich{dim("Today"), gap(4)}, tail(a.today()))
	week := cat(rich{dim("Week"), gap(5)}, spark(c.g, a.weekCosts()), rich{gap(3)}, tail(a.week()))
	plain := cat(rich{dim("Week"), gap(5)}, tail(a.week()))
	return []rich{today, firstFit(c.w, week, plain)}
}

func runningRows(a Account, w int) []rich {
	label := rich{dim("Running")}
	if len(a.Sessions) == 0 {
		return hang(label, 9, rich{dim("nothing right now")}, w)
	}
	var n [3]int
	for _, s := range a.Sessions {
		switch s.State {
		case StateWorking:
			n[0]++
		case StateWaiting:
			n[1]++
		default:
			n[2]++
		}
	}
	var parts []rich
	if n[0] > 0 {
		parts = append(parts, rich{good(fmt.Sprintf("%d working", n[0]))})
	}
	if n[1] > 0 {
		parts = append(parts, rich{warn(fmt.Sprintf("%d waiting for you", n[1]))})
	}
	if n[2] > 0 {
		parts = append(parts, rich{dim(fmt.Sprintf("%d idle", n[2]))})
	}
	body := rich{txt(plural(len(a.Sessions), "session") + ": ")}
	for i, p := range parts {
		if i > 0 {
			body = append(body, txt(", "))
		}
		body = append(body, p...)
	}
	return hang(label, 9, body, w)
}
