package reviews

import (
	"context"
	"testing"
)

func TestTickSavesNewAndSkippedReviewsAsOpenStampedWithNow(t *testing.T) {
	r := newRig(t)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := r.svc.Now()
	for _, id := range []string{"o-r-7-1111111", "o-r-8-2222222"} {
		got, ok, err := r.svc.Get(id)
		if err != nil || !ok {
			t.Fatalf("%s: get = %v %v", id, ok, err)
		}
		if got.State != "open" {
			t.Errorf("%s: state = %q, want open", id, got.State)
		}
		if got.StateCheckedAt == nil || !got.StateCheckedAt.Equal(want) {
			t.Errorf("%s: stateCheckedAt = %v, want %v", id, got.StateCheckedAt, want)
		}
	}
}
