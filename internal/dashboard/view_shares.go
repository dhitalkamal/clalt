package dashboard

import "fmt"

// sharesContent lists projects or models with each one's share of the cost
func sharesContent(c viewCtx, items []Share, projects bool, head []rich) content {
	total := 0.0
	for _, u := range items {
		total += u.Cost
	}
	build := func(barW int) [][]rich {
		rows := make([][]rich, len(items))
		for i, u := range items {
			pct := percentOf(u.Cost, total)
			rows[i] = []rich{{txt(u.Name)},
				cat(shareBar(c.g, float64(pct), barW), rich{gap(1), txt(fmt.Sprintf("%3d%%", pct))}),
				{txt(tokensText(u.Tokens))}, {txt(approx(u.Cost))}}
			if projects {
				rows[i] = append(rows[i], rich{txt(fmt.Sprint(u.Sessions))})
			}
		}
		return rows
	}
	// models carry tokens beside the share, and the cheaper model often has
	// more of them, so the header says what the share is of
	name, share := "model", "cost share"
	if projects {
		name, share = "project", "share"
	}
	cols := []column{{name, false}, {share, false}, {"tokens", true}, {"cost", true}}
	if projects {
		cols = append(cols, column{"sessions", true})
	}
	var tries []tableTry
	for _, t := range [][2]int{{12, 2}, {10, 2}, {8, 2}, {8, 1}, {6, 1}, {4, 1}} {
		tries = append(tries, tableTry{cols, build(t[0]), t[1]})
	}
	if projects {
		rows := build(4)
		for i := range rows {
			rows[i] = rows[i][:4]
		}
		tries = append(tries, tableTry{cols[:4], rows, 1})
	}
	hdr, lines := pickTable(c.w-2, tries)
	head = append(head, cat(rich{gap(2)}, hdr))
	body := append(cursorLines(c, lines), nil, rich{dim("Last 30 days. Share is by cost.")})
	return content{head: head, body: body, rows: len(lines), stride: 1, span: 1}
}
