package keychain

import (
	"bytes"
	"testing"
)

func TestFileStoreRoundTrip(t *testing.T) {
	f := NewFileStore(t.TempDir())

	// missing -> ErrNotFound
	if _, err := f.Get("svc", "acct"); err != ErrNotFound {
		t.Errorf("Get missing = %v, want ErrNotFound", err)
	}

	// set + get, including a service name with spaces (like the real live entry)
	if err := f.Set("Claude Code-credentials", "user", []byte("secret-bytes")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := f.Get("Claude Code-credentials", "user")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, []byte("secret-bytes")) {
		t.Errorf("Get = %q", got)
	}

	// list under a service
	_ = f.Set("clalt", "work", []byte("a"))
	_ = f.Set("clalt", "personal", []byte("b"))
	names, err := f.List("clalt")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 || names[0] != "personal" || names[1] != "work" {
		t.Errorf("List = %v, want [personal work]", names)
	}

	// delete
	if err := f.Delete("clalt", "work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f.Get("clalt", "work"); err != ErrNotFound {
		t.Errorf("after delete Get = %v, want ErrNotFound", err)
	}
	// deleting a missing item is not an error
	if err := f.Delete("clalt", "ghost"); err != nil {
		t.Errorf("Delete missing = %v, want nil", err)
	}
}
