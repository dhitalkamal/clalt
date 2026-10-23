package dashboard

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Run shows the dashboard until the user quits. a SIGINT counts as quitting.
func Run(ctx context.Context, src Source, act Actions, opt Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err := tea.NewProgram(New(ctx, src, act, opt), tea.WithContext(ctx)).Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}

// Render draws one frame of snap at w by h as plain text with no escape codes,
// for output that is not a terminal. opt picks the glyph set and the clock the
// times in snap are shown against. the texts of snap are cleaned first, like
// the program does for every load.
func Render(snap Snapshot, w, h int, opt Options) string {
	opt.NoColor = true
	m := newModel(context.Background(), nil, nil, opt)
	m.snap, m.have, m.loading = cleanSnapshot(snap), true, false
	m.loadedAt = m.now()
	m.w, m.h = w, h
	return ansi.Strip(m.refilter().render())
}
