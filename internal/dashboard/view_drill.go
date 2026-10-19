package dashboard

import "fmt"

func findProject(projects []Share, name string) (Share, bool) {
	for _, p := range projects {
		if p.Name == name {
			return p, true
		}
	}
	return Share{}, false
}

// drillContent is one project's activity: the trail and the project totals on
// top, then a row per day and model, newest first. Nothing in it is selectable,
// the rows only scroll.
func drillContent(c viewCtx, a Account, p Share) content {
	total := 0.0
	for _, s := range a.History.Projects {
		total += s.Cost
	}
	crumb := rich{dim("Projects / "), bold(p.Name)}.ellipsize(c.w, c.g.ellipsis)
	// cost, usage and sessions stay whole so a break never parts a number from its unit
	totals := []rich{
		{txt(approx(p.Cost)), dim(fmt.Sprintf(" (%d%% of cost)", percentOf(p.Cost, total)))},
		{txt(tokensText(p.Tokens)), dim(" tokens")},
		{txt(plural(p.Sessions, "session"))},
	}
	head := []rich{crumb}
	head = append(head, beside(rich{dim("Total")}, 8, flow(totals, 3, max(c.w-8, 1)))...)
	head = append(head, nil)
	if len(p.Activity) == 0 {
		return content{head: head, body: wrap(rich{dim("No activity recorded for this project.")}, c.w)}
	}

	rows := make([][]rich, len(p.Activity))
	for i, act := range p.Activity {
		rows[i] = []rich{{txt(act.Day)}, {txt(act.Model)}, {txt(fmt.Sprint(act.Sessions))},
			{txt(tokensText(act.Tokens))}, {txt(approx(act.Cost))}}
	}
	full := []column{{"day", false}, {"model", false}, {"sessions", true}, {"tokens", true}, {"cost", true}}
	short := []column{{"day", false}, {"model", false}, {"sess", true}, {"tokens", true}, {"cost", true}}
	hdr, lines := pickTable(c.w-2, []tableTry{{full, rows, 2}, {full, rows, 1}, {short, rows, 2}, {short, rows, 1}})
	head = append(head, cat(rich{gap(2)}, hdr))
	body := make([]rich, 0, len(lines)+2)
	for _, l := range lines {
		body = append(body, cat(rich{gap(2)}, l))
	}
	body = append(body, nil, rich{dim("Last 30 days, newest first.")})
	return content{head: head, body: body}
}
