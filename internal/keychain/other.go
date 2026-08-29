//go:build !darwin

package keychain

// newNative returns nil off macOS: there is no OS keychain clalt needs on
// Linux or Windows because Claude Code stores its live credential in the
// ~/.claude/.credentials.json file there, and clalt's own secrets live in the
// encrypted file vault. A nil Store means "no native keychain"; callers
// (livestore, MigrateVault) treat nil as file-only / nothing-to-migrate.
func newNative() Store { return nil }
