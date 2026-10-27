package wrapper

import (
	"io"
	"os"

	"golang.org/x/term"
)

// termState puts the terminal back the way it was when the wrapper started. an
// interactive claude changes the terminal while it runs (raw input, bracketed
// paste, mouse and focus reporting, the keyboard protocol, the alternate screen)
// and puts it back when it ends. it is not documented that it does so when it
// gets SIGTERM, and after a kill it cannot, so the wrapper does not rely on it.
type termState struct {
	fd     int
	state  *term.State
	out    io.Writer
	outTTY bool
}

// NewTerminal saves the settings of in, if it is a terminal. out is where the
// reset sequences go after a kill, and only when it is a terminal too.
func NewTerminal(in *os.File, out io.Writer) Terminal {
	t := &termState{out: out}
	if in != nil && term.IsTerminal(int(in.Fd())) {
		t.fd = int(in.Fd())
		if st, err := term.GetState(t.fd); err == nil {
			t.state = st
		}
	}
	if f, ok := out.(*os.File); ok {
		t.outTTY = term.IsTerminal(int(f.Fd()))
	}
	return t
}

// Restore puts the settings back. forced says claude was killed, so what it
// turned on in the terminal is turned off here.
func (t *termState) Restore(forced bool) {
	if t.state != nil {
		_ = term.Restore(t.fd, t.state)
	}
	if forced && t.outTTY {
		writeReset(t.out)
	}
}

// writeReset turns off what claude's interface turns on: bracketed paste, focus
// and mouse reporting, the kitty keyboard protocol (popped a few times, which
// is harmless when it was pushed fewer), the alternate screen, and shows the
// cursor.
func writeReset(w io.Writer) {
	_, _ = io.WriteString(w, "\x1b[?2004l\x1b[?1004l\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[<9u\x1b[?1049l\x1b[?25h\x1b[0m")
}
