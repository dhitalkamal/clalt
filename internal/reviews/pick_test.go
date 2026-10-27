package reviews

import "testing"

func pct(v float64) *float64 { return &v }

func TestPickMostRoom(t *testing.T) {
	c, ok := Pick([]Candidate{{ID: "a", FiveHourUsedPct: pct(60)}, {ID: "b", FiveHourUsedPct: pct(10)}, {ID: "c", FiveHourUsedPct: pct(30)}})
	if !ok || c.ID != "b" {
		t.Fatalf("%+v %v", c, ok)
	}
}

func TestPickKnownBeatsUnknownAndFullIsOut(t *testing.T) {
	c, ok := Pick([]Candidate{{ID: "u"}, {ID: "k", FiveHourUsedPct: pct(80)}})
	if !ok || c.ID != "k" {
		t.Fatalf("%+v", c)
	}
	if _, ok := Pick([]Candidate{{ID: "x", FiveHourUsedPct: pct(95)}}); ok {
		t.Fatal("an account at 95% has no room")
	}
	c, ok = Pick([]Candidate{{ID: "z"}, {ID: "y"}})
	if !ok || c.ID != "y" {
		t.Fatalf("unknown only: %+v (by id)", c)
	}
	if _, ok := Pick(nil); ok {
		t.Fatal("no accounts")
	}
}
