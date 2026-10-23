package wrapper

import (
	"errors"
	"os"
)

// terminate cannot ask a process on Windows to end: only a kill is possible
// there, and that would not let claude save anything. so a move is refused.
func terminate(p *os.Process) error {
	return errors.New("claude cannot be asked to end cleanly on windows, so a session cannot be moved there")
}

func terminated(error) bool { return false }
