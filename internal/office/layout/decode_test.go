package layout

import (
	"strings"
	"testing"
)

func TestDecodeAcceptsAValidLayout(t *testing.T) {
	l, err := Decode(mustJSON(t, valid()))
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "mine" || len(l.Rooms) != 3 {
		t.Errorf("decoded %+v", l)
	}
}

func TestDecodeRefusals(t *testing.T) {
	good := string(mustJSON(t, valid()))
	cases := map[string]string{
		"unknown key":     strings.Replace(good, `"v":1`, `"v":1,"extra":true`, 1),
		"unknown nested":  strings.Replace(good, `"grid":0.5`, `"grid":0.5,"walls2":{}`, 1),
		"trailing value":  good + `{}`,
		"not json":        `nope`,
		"empty":           ``,
		"oversize":        good[:len(good)-1] + `,"pad":"` + strings.Repeat("a", MaxBytes) + `"}`,
		"wrong type":      strings.Replace(good, `"v":1`, `"v":"1"`, 1),
		"null for a list": strings.Replace(good, `"rooms":[`, `"rooms":null,"x":[`, 1),
	}
	for name, raw := range cases {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Errorf("%s: decoded without an error", name)
		}
	}
}

func TestDecodeCleansStrings(t *testing.T) {
	l := valid()
	l.Title = "  My\x07 \u202eoffice\n\x1b[31m  "
	l.Rooms[0].Label = "Lob\u200bby"
	got, err := Decode(mustJSON(t, l))
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "My office" || got.Rooms[0].Label != "Lob by" {
		t.Errorf("title %q label %q", got.Title, got.Rooms[0].Label)
	}
}

func TestDecodeNormalizesNilListsToEmpty(t *testing.T) {
	l := valid()
	l.Furniture = nil
	got, err := Decode(mustJSON(t, l))
	if err != nil {
		t.Fatal(err)
	}
	if got.Furniture == nil {
		t.Error("furniture stays nil, the page would read null")
	}
}
