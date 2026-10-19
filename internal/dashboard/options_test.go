package dashboard

import (
	"testing"
	"time"
)

func TestEnvOptions(t *testing.T) {
	env := func(vals map[string]string) func(string) string {
		return func(k string) string { return vals[k] }
	}
	cases := []struct {
		name string
		env  map[string]string
		want Options
	}{
		{"nothing set", nil, Options{}},
		{"ascii on", map[string]string{"CLALT_ASCII": "1"}, Options{ASCII: true}},
		{"ascii needs exactly 1", map[string]string{"CLALT_ASCII": "yes"}, Options{}},
		{"no color set", map[string]string{"NO_COLOR": "1"}, Options{NoColor: true}},
		{"no color any non-empty value", map[string]string{"NO_COLOR": "true"}, Options{NoColor: true}},
		{"empty no color is off", map[string]string{"NO_COLOR": ""}, Options{}},
		{"both", map[string]string{"CLALT_ASCII": "1", "NO_COLOR": "1"}, Options{ASCII: true, NoColor: true}},
	}
	for _, c := range cases {
		if got := EnvOptions(env(c.env)); got.ASCII != c.want.ASCII || got.NoColor != c.want.NoColor {
			t.Errorf("%s: got %+v, want ascii %v no color %v", c.name, got, c.want.ASCII, c.want.NoColor)
		}
	}
}

func TestOptionDefaults(t *testing.T) {
	got := Options{}.withDefaults()
	if got.Refresh != 5*time.Second || got.FlashFor != 3*time.Second || got.LoadTimeout != 30*time.Second {
		t.Errorf("defaults = %+v", got)
	}
	if got.Now == nil || time.Since(got.Now()) > time.Minute {
		t.Error("the default clock is the real one")
	}
	set := Options{Refresh: time.Second, FlashFor: time.Millisecond, LoadTimeout: time.Minute}.withDefaults()
	if set.Refresh != time.Second || set.FlashFor != time.Millisecond || set.LoadTimeout != time.Minute {
		t.Errorf("explicit values stay: %+v", set)
	}
}
