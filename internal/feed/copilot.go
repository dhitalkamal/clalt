package feed

import "strings"

const copilotPrefix = "cp-"

// CopilotHandle is the key of a GitHub Copilot CLI session, from its own
// session id. same id rules as ExtHandle, since it names a file too.
func CopilotHandle(copilotSessionID string) (string, bool) {
	if !validExtID(copilotSessionID) {
		return "", false
	}
	return copilotPrefix + copilotSessionID, true
}

// IsCopilot says whether handle is the key of a Copilot session.
func IsCopilot(handle string) bool {
	id, ok := strings.CutPrefix(handle, copilotPrefix)
	return ok && validExtID(id)
}
