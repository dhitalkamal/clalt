// Package terminal opens a session in a new tab or window and brings a running
// session's terminal to the front, using only the kitty socket or tmux server
// that is already there.
package terminal

import "time"

// Kind is the kind of terminal a session runs in.
type Kind string

// The kinds a session registry records.
const (
	Kitty Kind = "kitty"
	Tmux  Kind = "tmux"
	IDE   Kind = "ide"
	Plain Kind = "terminal"
)

// Location is where a session's terminal is, the way the session registry
// records it.
type Location struct {
	Kind          Kind
	KittyWindow   int    // KITTY_WINDOW_ID
	KittyListenOn string // KITTY_LISTEN_ON as it was
	TmuxSocket    string // the socket path in TMUX
	TmuxPane      string // TMUX_PANE, like %5
	IDE           string // the name of the IDE
}

// Session is the part of a running session's registry record that jump and
// resume read.
type Session struct {
	Handle          string
	Account         string // the account id
	ClaudeSessionID string // empty until claude has said which it is
	Cwd             string
	Location        Location
	StartedAt       time.Time
}
