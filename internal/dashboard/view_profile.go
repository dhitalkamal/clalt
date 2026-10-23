package dashboard

import "time"

const profileLabelW = 11

// signedText says how long ago the account signed in, when that is known
func signedText(l Login, now time.Time) string {
	switch days := daysSince(l.SignedInAt, now); {
	case l.SignedInAt.IsZero():
		return "signed in"
	case days == 0:
		return "signed in today"
	default:
		return "signed in " + plural(days, "day") + " ago"
	}
}

func profileContent(c viewCtx, a Account) content {
	if !a.SignedIn {
		return signedOutContent(c)
	}
	p := a.Profile
	settings := "shared with ~/.claude (skills, agents, MCP)"
	if !p.SharedSettings {
		settings = "own copy, not shared with ~/.claude"
	}
	claude := rich{txt(p.ClaudeVersion)}
	if p.ClaudeVersion == "" {
		claude = rich{warn("not found")}
	}
	login := rich{txt(signedText(a.Login, c.now))}
	if !a.Login.DueAt.IsZero() {
		days := daysLeft(a.Login.DueAt, c.now)
		due := "re-login due in " + plural(days, "day")
		if days <= 0 {
			due = "re-login is due now"
		}
		login = append(login, txt(", "), span{lvTone(loginLevel(days), toneText), due})
	}
	feed := rich{warn("not installed")}
	if p.FeedInstalled {
		feed = rich{txt("installed, no reading yet")}
		if !a.Limits.ReadAt.IsZero() {
			feed = rich{txt("installed, last reading " + agoText(c.now.Sub(a.Limits.ReadAt)))}
		}
	}
	used := rich{txt("never")}
	if !a.LastUsedAt.IsZero() {
		used = rich{txt(agoText(c.now.Sub(a.LastUsedAt)))}
		if a.LastIn != "" {
			used = rich{txt(agoText(c.now.Sub(a.LastUsedAt)) + " in " + a.LastIn)}
		}
	}
	items := []struct {
		label string
		value rich
	}{
		{"Folder", rich{txt(p.Folder)}},
		{"Claude", claude},
		{"Settings", rich{txt(settings)}},
		{"Login", login},
		{"Usage feed", feed},
		{"Last used", used},
	}
	if p.Gateway != "" {
		items = append(items, struct {
			label string
			value rich
		}{"Gateway", rich{txt("running on " + p.Gateway)}})
	}
	var b []rich
	for _, it := range items {
		b = append(b, hang(rich{dim(it.label)}, profileLabelW, it.value, c.w)...)
	}
	return content{body: b}
}
