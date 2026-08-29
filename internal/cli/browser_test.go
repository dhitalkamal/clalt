package cli

import "testing"

func TestBrowserCommand(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantArg0 string
	}{
		{"darwin", "open", "https://x"},
		{"linux", "xdg-open", "https://x"},
		{"windows", "rundll32", "url.dll,FileProtocolHandler"},
		{"freebsd", "xdg-open", "https://x"}, // any other unix falls back to xdg-open
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			name, args := browserCommand(tc.goos, "https://x")
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if len(args) == 0 || args[0] != tc.wantArg0 {
				t.Errorf("args = %v, want first %q", args, tc.wantArg0)
			}
			// the url must always be passed to the opener somewhere
			found := false
			for _, a := range args {
				if a == "https://x" {
					found = true
				}
			}
			if !found {
				t.Errorf("url not in args %v", args)
			}
		})
	}
}
