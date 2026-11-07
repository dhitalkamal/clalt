package feed

import (
	"strings"

	"github.com/dhitalkamal/clalt/internal/registry"
)

const extPrefix = "ext-"

// ExtHandle is the key of a session that clalt run did not start, from claude's
// own session id. it names a file, so only letters, digits and hyphens, at most
// 64 of them, are let through (claude's ids are UUIDs).
func ExtHandle(claudeSessionID string) (string, bool) {
	if !validExtID(claudeSessionID) {
		return "", false
	}
	return extPrefix + claudeSessionID, true
}

func validExtID(id string) bool {
	if id == "" || len(id) > 60 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// IsExt says whether handle is the key of an ext session.
func IsExt(handle string) bool {
	id, ok := strings.CutPrefix(handle, extPrefix)
	return ok && validExtID(id)
}

// ValidKey says whether handle may name a session file: the handle of a clalt
// run session, an ext key or a Copilot key.
func ValidKey(handle string) bool {
	return registry.ValidHandle(handle) || IsExt(handle) || IsCopilot(handle)
}
