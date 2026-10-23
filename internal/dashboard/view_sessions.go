package dashboard

import "fmt"

// marker, space, state word and two spaces
const sessionIndent = 11

func stateTone(s SessionState) tone {
	switch s {
	case StateWorking:
		return toneGood
	case StateWaiting:
		return toneWarn
	}
	return toneDim
}

func sessionsContent(c viewCtx, a Account) content {
	if !a.SignedIn {
		return signedOutContent(c)
	}
	if len(a.Sessions) == 0 {
		return emptyContent(c.w, "No sessions running. Press ", "enter", " to start claude as "+a.Label+".")
	}
	var b []rich
	for i, s := range a.Sessions {
		b = append(b, sessionRows(c, s, i == c.row)...)
		b = append(b, nil)
	}
	b = append(b, rich{bold("x"), dim(" stops the selected session.")})
	return content{body: b, rows: len(a.Sessions), stride: 4, span: 3}
}

func sessionRows(c viewCtx, s Session, selected bool) []rich {
	mark, mt, pt := " ", toneText, toneText
	if selected {
		mark, mt = c.g.cursor, toneDim
		if c.focused {
			mt, pt = toneBold, toneBold
		}
	}
	state := span{stateTone(s.State), string(s.State)}
	where := rich{span{pt, s.Folder}}
	if s.Branch != "" {
		where = append(where, dim(" ("+s.Branch+")"))
	}
	l1 := cat(rich{span{mt, mark}, gap(1), state, gap(sessionIndent - 2 - len(s.State))}, where).ellipsize(c.w, c.g.ellipsis)

	indent := rich{gap(sessionIndent)}
	model := s.Model
	if s.Effort != "" {
		model += " " + s.Effort
	}
	l2 := cat(indent, rich{txt(model)})
	if s.ContextSize > 0 {
		ctx := tokensText(s.ContextUsed) + " of " + ctxSizeText(s.ContextSize)
		mini := usageBar(c.g, float64(s.ContextUsed)/float64(s.ContextSize)*100, 8)
		l2 = firstFit(c.w,
			cat(indent, rich{txt(model), gap(3), dim("context " + ctx), gap(3)}, mini),
			cat(indent, rich{txt(model), gap(2), dim("ctx " + ctx), gap(2)}, mini),
			cat(indent, rich{txt(model), gap(2), dim("ctx " + ctx)}),
			cat(indent, rich{txt(model)}),
		)
	}
	elapsed := "-"
	if !s.StartedAt.IsZero() {
		elapsed = elapsedText(c.now.Sub(s.StartedAt))
	}
	facts := rich{txt(elapsed), gap(3)}
	// no lines changed is also what a source that does not know them says
	if s.Added+s.Removed > 0 {
		facts = append(facts, txt(fmt.Sprintf("+%d -%d lines", s.Added, s.Removed)), gap(3))
	}
	l3 := cat(indent, facts, rich{txt(approx(s.Cost))}).clip(c.w)
	return []rich{l1, l2, l3}
}
