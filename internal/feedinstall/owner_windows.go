//go:build windows

package feedinstall

// fileOwnedByUser: windows has no uid to compare, the file's ACL decides at write.
func fileOwnedByUser(string) bool { return true }
