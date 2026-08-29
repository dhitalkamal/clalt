// Package signing reports how the running clalt binary is code-signed on macOS.
// This matters because macOS Keychain access control identifies an app by its
// code-signing identity: an ad-hoc (go build default) signature gets a fresh
// code hash every rebuild, so a keychain "always allow" grant never persists and
// clalt is re-prompted for the login password on every run. A stable identity
// (self-signed code-signing cert or Developer ID) keeps the grant sticky.
package signing

import (
	"os/exec"
	"strings"
)

// Kind classifies a binary's code signature.
type Kind int

const (
	// Unknown means the signature could not be classified.
	Unknown Kind = iota
	// Unsigned means the binary carries no signature at all.
	Unsigned
	// AdHoc means an ad-hoc / linker-signed signature: identity is not stable
	// across rebuilds, so keychain trust grants do not persist.
	AdHoc
	// Stable means a persistent identity (self-signed cert or Developer ID),
	// so a keychain "always allow" grant sticks across rebuilds.
	Stable
)

// Stable reports whether this Kind keeps keychain trust grants across rebuilds.
func (k Kind) Stable() bool { return k == Stable }

// String renders the Kind for human output.
func (k Kind) String() string {
	switch k {
	case Unsigned:
		return "unsigned"
	case AdHoc:
		return "adhoc"
	case Stable:
		return "stable"
	default:
		return "unknown"
	}
}

// Classify inspects the combined output of `codesign -dvv <path>` (codesign
// writes these details to stderr) and reports the signature kind.
func Classify(codesignOutput string) Kind {
	if strings.TrimSpace(codesignOutput) == "" {
		return Unknown
	}
	if strings.Contains(codesignOutput, "code object is not signed") {
		return Unsigned
	}
	// ad-hoc must be checked before Authority: an ad-hoc binary has no
	// Authority line, but this keeps the intent explicit.
	if strings.Contains(codesignOutput, "Signature=adhoc") || strings.Contains(codesignOutput, "adhoc,") {
		return AdHoc
	}
	// a real identity always prints at least one Authority line.
	if strings.Contains(codesignOutput, "Authority=") {
		return Stable
	}
	return Unknown
}

// Running classifies the currently executing binary at path. It shells out to
// codesign (macOS only); on any error it returns Unknown so callers degrade
// gracefully rather than failing a diagnostic.
func Running(path string) Kind {
	out, err := exec.Command("codesign", "-dvv", path).CombinedOutput()
	if err != nil && len(out) == 0 {
		return Unknown
	}
	return Classify(string(out))
}
