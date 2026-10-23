package wrapper

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTheResetSequenceUndoesWhatAnInterfaceLikeClaudesTurnsOn(t *testing.T) {
	var out bytes.Buffer

	writeReset(&out)

	for name, seq := range map[string]string{
		"bracketed paste off": "\x1b[?2004l", "focus reporting off": "\x1b[?1004l", "mouse off": "\x1b[?1000l",
		"sgr mouse off": "\x1b[?1006l", "cursor shown": "\x1b[?25h", "main screen": "\x1b[?1049l", "keyboard mode popped": "\x1b[<",
	} {
		if !strings.Contains(out.String(), seq) {
			t.Errorf("the reset lacks %s (%q)", name, seq)
		}
	}
}

// stdin that is a pipe has no terminal settings to save, and nothing to put back
func TestATerminalThatIsNotOneDoesNothing(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	var out bytes.Buffer
	tm := NewTerminal(r, &out)

	tm.Restore(false)
	tm.Restore(true)

	if out.Len() != 0 {
		t.Errorf("wrote %q to something that is not a terminal", out.String())
	}
}
