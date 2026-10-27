package profile

// Kill terminates the process behind a session and prunes its record. The record
// is always removed (a dead process should not linger in the list) even if the
// signal itself fails, and the signal error is returned for the caller to report.
func Kill(dir string, pid int) error {
	err := killProcess(pid)
	_ = RemoveSession(dir, pid)
	return err
}
