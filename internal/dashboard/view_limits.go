package dashboard

import "fmt"

func limitsContent(c viewCtx, a Account) content {
	if !a.SignedIn {
		return signedOutContent(c)
	}
	lim := a.Limits
	var b []rich
	if chart := chartRows(c.g, lim.Readings, lim.Days, c.w, clampInt(c.h-8, 3, 6)); chart != nil {
		now := rich{dim("now "), span{pctTone(lim.Five.Percent), fmt.Sprintf("%d%%", lim.Five.Percent)}}
		title := rich{dim("5h window, last " + plural(len(lim.Days), "day"))}
		b = append(b, alignRight(title, now, chart[0].width()))
		b = append(b, chart...)
	} else {
		b = append(b, rich{dim("No readings yet.")})
	}
	b = append(b, nil)
	for _, p := range []struct {
		label  string
		x      Window
		weekly bool
	}{{"7d", lim.Seven, true}, {"5h", lim.Five, false}} {
		if pace := paceLong(p.x, p.weekly, c.now); pace != "" {
			b = append(b, hang(rich{dim(p.label)}, 4, rich{span{lvTone(paceLevel(p.x, c.now), toneText), pace}}, c.w)...)
		}
	}
	return content{body: b}
}
