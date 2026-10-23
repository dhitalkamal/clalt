package demo

import (
	"context"
	"testing"
)

func TestTheDemoHasARemovedAccount(t *testing.T) {
	snap, err := Source(atEpoch).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Removed) != 1 || snap.Removed[0].Label == "" || snap.Removed[0].Email == "" {
		t.Errorf("removed = %+v", snap.Removed)
	}
}
