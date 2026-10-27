package office

import (
	"strings"

	"github.com/dhitalkamal/clalt/internal/registry"
)

// actionsFor is what can be done to a session of this record. a move needs the
// id claude has to resume with, a focus needs kitty or tmux, and moving and
// stopping end claude with a signal that windows does not have.
func actionsFor(r registry.Record, goos string) ActionsOut {
	out := ActionsOut{Move: true, Focus: true, Stop: true}
	var why []string
	if goos == "windows" {
		out.Move, out.Stop = false, false
		why = append(why, "moving and stopping are not supported on windows")
	} else if r.ClaudeSessionID == "" {
		out.Move = false
		why = append(why, "claude has not reported its session id yet, so it cannot move")
	}
	if k := r.Terminal.Kind; k != "kitty" && k != "tmux" {
		out.Focus = false
		why = append(why, "its terminal cannot be brought to the front")
	}
	out.Why = strings.Join(why, "; ")
	return out
}
