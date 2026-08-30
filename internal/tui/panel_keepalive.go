package tui

// renderKeepalive draws the keepalive panel: whether the background refresh job
// is installed, its backend, and a token-freshness summary.
func (m dashModel) renderKeepalive(innerW, innerH int) string {
	focused := m.focus == focusKeepalive
	ka := m.data.Keepalive

	status := dimStyle.Render("off")
	if ka.Installed {
		status = okStyle.Render("active")
	}
	backend := ""
	if ka.Backend != "" {
		backend = dimStyle.Render("  " + ka.Backend)
	}
	line1 := status + backend

	fresh := ka.Accounts - ka.NearExpiry
	if fresh < 0 {
		fresh = 0
	}
	line2 := dimStyle.Render(itoa(fresh) + "/" + itoa(ka.Accounts) + " tokens fresh")
	if ka.NearExpiry > 0 {
		line2 = lipglossFg(colWarn).Render(itoa(ka.NearExpiry) + " near expiry")
	}

	body := clampLine(line1, innerW) + "\n" + clampLine(line2, innerW)
	return panelBox("Keepalive", innerW, innerH, body, focused)
}
