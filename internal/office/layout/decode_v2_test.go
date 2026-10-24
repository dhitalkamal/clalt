package layout

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeV2RoundTrip(t *testing.T) {
	l := valid2()
	raw, err := Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"grid"`, `"size"`, `"walls"`, `"rooms"`, `"furniture"`, `"door"`, `"floorColor"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("a v2 file holds the v1 key %s", key)
		}
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(got, nil); err != nil {
		t.Fatal(err)
	}
	if got.Name != "garden" || len(got.Pavilions) != 3 || got.Site.River == nil || got.Eye == nil {
		t.Errorf("decoded %+v", got)
	}
}

func TestDecodeV2NullRiverAndEmptyLists(t *testing.T) {
	l := valid2()
	l.Site.River, l.Site.Ponds, l.Site.Paths, l.Site.Bridges, l.Site.Props = nil, nil, nil, nil, nil
	l.Overflow, l.Eye = nil, nil
	raw, err := Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{`"river": null`, `"ponds": []`, `"paths": []`, `"overflow": []`} {
		if !strings.Contains(s, want) {
			t.Errorf("encoded layout misses %s", want)
		}
	}
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRulesPerVersion(t *testing.T) {
	v1 := string(mustJSON(t, valid()))
	v2raw, err := Encode(valid2())
	if err != nil {
		t.Fatal(err)
	}
	v2 := string(v2raw)
	cases := map[string]string{
		"v1 with template":  strings.Replace(v1, `"v":1`, `"v":1,"template":"rows"`, 1),
		"v1 with site":      strings.Replace(v1, `"v":1`, `"v":1,"site":null`, 1),
		"v1 with pavilions": strings.Replace(v1, `"v":1`, `"v":1,"pavilions":[]`, 1),
		"v2 with grid":      strings.Replace(v2, `"v": 2`, `"v": 2, "grid": 0.5`, 1),
		"v2 with rooms":     strings.Replace(v2, `"v": 2`, `"v": 2, "rooms": []`, 1),
		"v2 with furniture": strings.Replace(v2, `"v": 2`, `"v": 2, "furniture": []`, 1),
		"v2 unknown key":    strings.Replace(v2, `"v": 2`, `"v": 2, "extra": 1`, 1),
		"v2 nested unknown": strings.Replace(v2, `"fence": true`, `"fence": true, "moat": 1`, 1),
		"v2 pavilion extra": strings.Replace(v2, `"type": "lobby"`, `"type": "lobby", "hat": 1`, 1),
		"v2 wrong type":     strings.Replace(v2, `"fence": true`, `"fence": "yes"`, 1),
	}
	for name, raw := range cases {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	if _, err := Decode([]byte(v1)); err != nil {
		t.Errorf("a v1 file stopped decoding: %v", err)
	}
}

func TestDecodeV2CleansStrings(t *testing.T) {
	l := valid2()
	l.Title = "  Big \x1b[31mgarden\x1b[0m\n  "
	l.Pavilions[2].Label = "WO\tRK"
	raw, _ := json.Marshal(l)
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Big garden" || got.Pavilions[2].Label != "WO RK" {
		t.Errorf("title %q label %q", got.Title, got.Pavilions[2].Label)
	}
}

func TestV1EncodingUnchangedByV2Fields(t *testing.T) {
	raw, err := Encode(valid())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"template"`, `"site"`, `"pavilions"`, `"overflow"`, `"eye"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("a v1 file holds the v2 key %s", key)
		}
	}
}
