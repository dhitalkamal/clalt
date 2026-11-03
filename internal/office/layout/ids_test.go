package layout

import "testing"

func TestValidIDAndAccount(t *testing.T) {
	for id, want := range map[string]bool{"cabin-1": true, "a": true, "": false, "Cabin": false, "a b": false, "a_b": false} {
		if got := ValidID(id); got != want {
			t.Errorf("ValidID(%q) = %v", id, got)
		}
	}
	if ValidID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Error("33 chars passed")
	}
	for id, want := range map[string]bool{"work": true, "Me.x_y-1": true, "": false, "a b": false, "a/b": false} {
		if got := ValidAccount(id); got != want {
			t.Errorf("ValidAccount(%q) = %v", id, got)
		}
	}
}
