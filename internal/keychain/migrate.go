package keychain

// MigrateVault copies every item under service from src into dst, but only when
// dst currently has none. It moves clalt's per-account secrets out of the macOS
// Keychain (src) into the local encrypted vault (dst) on the first run after the
// vault was introduced. The one-time read from the Keychain may prompt for the
// login password once per account; after that the vault serves every read with
// no prompt. Legacy Keychain items are left in place (harmless; nothing reads
// them anymore) so the migration is non-destructive and safely re-runnable.
func MigrateVault(src, dst Store, service string) (int, error) {
	if src == nil {
		return 0, nil // no source keychain (off macOS): nothing to migrate
	}
	existing, _ := dst.List(service)
	if len(existing) > 0 {
		return 0, nil // already migrated
	}
	names, err := src.List(service)
	if err != nil || len(names) == 0 {
		return 0, nil // nothing to migrate (or src unavailable)
	}
	migrated := 0
	for _, name := range names {
		data, err := src.Get(service, name)
		if err != nil {
			continue
		}
		if err := dst.Set(service, name, data); err != nil {
			continue
		}
		migrated++
	}
	return migrated, nil
}
