package cli

import "testing"

func TestValidateMagicLink(t *testing.T) {
	cases := []struct {
		name    string
		link    string
		wantErr bool
	}{
		{"claude.ai host", "https://claude.ai/x", false},
		{"anthropic.com host", "https://anthropic.com/x", false},
		{"anthropic.com subdomain", "https://accounts.anthropic.com/x", false},
		{"http scheme rejected", "http://claude.ai/x", true},
		{"unrelated host rejected", "https://evil.com/x", true},
		{"host merely containing anthropic.com rejected", "https://notanthropic.com/x", true},
		{"host with anthropic.com as suffix but no dot rejected", "https://evilanthropic.com/x", true},
		{"malformed url rejected", "://bad", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMagicLink(tc.link)
			if tc.wantErr && err == nil {
				t.Errorf("validateMagicLink(%q) = nil, want error", tc.link)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateMagicLink(%q) = %v, want nil", tc.link, err)
			}
		})
	}
}

func TestParseLoginArgs(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		wantName      string
		wantMagicLink string
		wantErr       bool
	}{
		{"bare name", []string{"acct"}, "acct", "", false},
		{"name plus magic link", []string{"acct", "--magic-link", "https://claude.ai/x"}, "acct", "https://claude.ai/x", false},
		{"magic link before name", []string{"--magic-link", "https://claude.ai/x", "acct"}, "acct", "https://claude.ai/x", false},
		{"magic link missing value errors", []string{"acct", "--magic-link"}, "", "", true},
		{"missing name errors", []string{}, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, magicLink, err := parseLoginArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseLoginArgs(%v) = nil error, want error", tc.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLoginArgs(%v) unexpected error: %v", tc.args, err)
			}
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if magicLink != tc.wantMagicLink {
				t.Errorf("magicLink = %q, want %q", magicLink, tc.wantMagicLink)
			}
		})
	}
}
