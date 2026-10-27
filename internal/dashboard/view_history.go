package dashboard

import "fmt"

// the days table shows the latest ten, the sparkline and total cover the month
const dayTableRows = 10

type tableTry struct {
	cols []column
	rows [][]rich
	gap  int
}

// pickTable takes the first layout that fits, or the last one when none do
func pickTable(w int, tries []tableTry) (rich, []rich) {
	for _, t := range tries {
		if tableWidth(t.cols, t.rows, t.gap) <= w {
			return table(t.cols, t.rows, t.gap)
		}
	}
	t := tries[len(tries)-1]
	return table(t.cols, t.rows, t.gap)
}

// cursorLines prefixes each row with the cursor column
func cursorLines(c viewCtx, lines []rich) []rich {
	out := make([]rich, len(lines))
	for i, l := range lines {
		mark, mt := " ", toneText
		if i == c.row {
			mark, mt = c.g.cursor, toneDim
			if c.focused {
				mt = toneBold
				l = boldText(l)
			}
		}
		out[i] = cat(rich{span{mt, mark}, gap(1)}, l)
	}
	return out
}

func boldText(r rich) rich {
	out := make(rich, len(r))
	for i, sp := range r {
		if sp.t == toneText {
			sp.t = toneBold
		}
		out[i] = sp
	}
	return out
}

func historyContent(c viewCtx, a Account) content {
	if !a.SignedIn {
		return signedOutContent(c)
	}
	head := []rich{tabStrip(c.g, subNames, c.sub, 0, 2), nil}
	h := a.History
	if len(h.Days) == 0 && len(h.Projects) == 0 && len(h.Models) == 0 {
		msg := prompt("No usage history yet. Press ", "enter", " to start claude as "+a.Label+".")
		return content{head: head, body: wrap(msg, c.w)}
	}
	if c.sub == 1 && c.drill != "" {
		if p, ok := findProject(h.Projects, c.drill); ok {
			return drillContent(c, a, p)
		}
	}
	switch c.sub {
	case 1:
		return sharesContent(c, h.Projects, true, head)
	case 2:
		return sharesContent(c, h.Models, false, head)
	}
	return daysContent(c, a, head)
}

func daysContent(c viewCtx, a Account, head []rich) content {
	shown := firstDays(a.History.Days, dayTableRows)
	rows := make([][]rich, len(shown))
	for i, d := range shown {
		t := toneText
		if d.Cost == 0 {
			t = toneDim
		}
		cell := func(s string) rich { return rich{span{t, s}} }
		rows[i] = []rich{cell(d.Label), cell(cellMillions(d.Input)), cell(cellMillions(d.Output)),
			cell(cellMillions(d.CacheRead)), cell(cellMillions(d.CacheWrite)), cell(approx(d.Cost)), cell(fmt.Sprint(d.Sessions))}
	}
	full := []column{{"day", false}, {"input", true}, {"output", true}, {"cache rd", true},
		{"cache wr", true}, {"cost", true}, {"sessions", true}}
	short := []column{{"day", false}, {"in", true}, {"out", true}, {"rd", true},
		{"wr", true}, {"cost", true}, {"sess", true}}
	hdr, lines := pickTable(c.w-2, []tableTry{{full, rows, 2}, {full, rows, 1}, {short, rows, 2}, {short, rows, 1}})
	head = append(head, cat(rich{gap(2)}, hdr))
	body := cursorLines(c, lines)

	series := a.monthCosts()
	total := approx(a.monthCost())
	label := "last " + plural(len(series), "day")
	if bars := clampInt(c.w-(2+len(label)+2+len(total)), 8, len(series)); bars < len(series) {
		series = resample(series, bars)
	}
	body = append(body, nil, cat(spark(c.g, series), rich{gap(2), dim(label), gap(2), txt(total)}))
	return content{head: head, body: body, rows: len(lines), stride: 1, span: 1}
}
