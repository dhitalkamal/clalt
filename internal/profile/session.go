package profile

import "github.com/dhitalkamal/clalt/internal/registry"

// Session is a record of the session registry. the names here are the ones the
// old dashboard and the tests of earlier versions use, kept so that they keep
// compiling; internal/registry is where the code is.
type Session = registry.Record

// WriteSession records a running session. a record that has no handle yet gets
// one, so a caller from before handles still works.
func WriteSession(dir string, s Session) error {
	if s.Handle == "" {
		s.Handle = registry.NewHandle()
	}
	return registry.Registry{Dir: dir}.Create(s)
}

// RemoveSession clears the records of a process (missing is not an error).
func RemoveSession(dir string, pid int) error {
	return registry.Registry{Dir: dir}.RemovePID(pid)
}

// ListSessions returns live sessions, pruning records whose process has exited.
func ListSessions(dir string) ([]Session, error) {
	return registry.Registry{Dir: dir}.List()
}
