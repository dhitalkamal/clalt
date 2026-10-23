package claudecli

import "testing"

func TestHelperFakeOK(t *testing.T) {
	tmp := t.TempDir()
	for name, c := range map[string]struct {
		fake string
		ok   bool
	}{
		"empty":        {"", false},
		"real claude":  {"/usr/local/bin/claude", false},
		"other name":   {tmp + "/sh", false},
		"fake in temp": {tmp + "/claude", true},
		"guard":        {tmp + "/claude-guard", true},
	} {
		if err := helperFakeOK(c.fake, tmp); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok %v", name, err, c.ok)
		}
	}
}
