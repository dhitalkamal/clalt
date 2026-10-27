package feedinstall

// fakeOwner makes the owner check answer ok for every file and returns the undo.
func fakeOwner(ok bool) func() {
	prev := ownedByUser
	ownedByUser = func(string) bool { return ok }
	return func() { ownedByUser = prev }
}
