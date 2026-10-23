// Package registry keeps one record per running `clalt run` session, keyed by a
// random handle that clalt passes to claude as CLALT_SESSION. the feed, the
// dashboard and the commands that move or jump to a session find it by that
// handle.
package registry

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Record is one running session. Account, Cwd, StartedAt and PID keep the names
// of the record of earlier versions, which the old dashboard still reads.
type Record struct {
	Handle          string `json:"handle"`
	Account         string `json:"account"`
	Folder          string `json:"folder"`
	WrapperPID      int    `json:"wrapperPid"`
	PID             int    `json:"pid"`
	ClaudeSessionID string `json:"claudeSessionId,omitempty"`
	// AttributedAccount is the account the session log last put ClaudeSessionID
	// under. a move keeps the id and changes the account.
	AttributedAccount string    `json:"attributedAccount,omitempty"`
	Cwd               string    `json:"cwd"`
	Terminal          Terminal  `json:"terminal"`
	StartedAt         time.Time `json:"startedAt"`
}

// Terminal says where the session's terminal is, as far as the environment of
// clalt run told. only the facts that were present are set, whatever the kind.
type Terminal struct {
	Kind          string `json:"kind"`
	KittyWindowID string `json:"kittyWindowId,omitempty"`
	KittyListenOn string `json:"kittyListenOn,omitempty"`
	TmuxSocket    string `json:"tmuxSocket,omitempty"`
	TmuxPane      string `json:"tmuxPane,omitempty"`
	IDE           string `json:"ide,omitempty"`
}

const handleLen = 16

// NewHandle returns a random 16 character hex handle.
func NewHandle() string {
	var b [handleLen / 2]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("no randomness: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// ValidHandle says whether s is a handle NewHandle could have made. the handle
// comes from the environment and names a file, so nothing else is let through.
func ValidHandle(s string) bool {
	if len(s) != handleLen {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
