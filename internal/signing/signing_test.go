package signing

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want Kind
	}{
		{
			// what `codesign -dvv` prints for a `go build` binary: the
			// linker-applied ad-hoc signature. this identity changes every
			// rebuild, so a keychain "always allow" never persists.
			name: "adhoc linker-signed",
			out: "Executable=/Users/x/go/bin/clalt\n" +
				"Identifier=a.out\n" +
				"Format=Mach-O thin (arm64)\n" +
				"CodeDirectory v=20400 flags=0x20002(adhoc,linker-signed)\n" +
				"Signature=adhoc\n",
			want: AdHoc,
		},
		{
			// a self-signed code-signing identity: stable Authority, no adhoc.
			name: "self signed identity",
			out: "Executable=/Users/x/bin/clalt\n" +
				"Identifier=com.clalt.cli\n" +
				"Authority=clalt-codesign\n" +
				"Signature size=1234\n",
			want: Stable,
		},
		{
			// a real Developer ID identity.
			name: "developer id",
			out: "Executable=/Users/x/bin/clalt\n" +
				"Authority=Developer ID Application: Example (TEAMID)\n" +
				"TeamIdentifier=TEAMID\n",
			want: Stable,
		},
		{
			name: "unsigned",
			out:  "/Users/x/bin/clalt: code object is not signed at all\n",
			want: Unsigned,
		},
		{
			name: "empty is unknown",
			out:  "",
			want: Unknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.out); got != tc.want {
				t.Fatalf("Classify(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestKindStable(t *testing.T) {
	if AdHoc.Stable() {
		t.Fatal("adhoc must not report stable")
	}
	if Unsigned.Stable() {
		t.Fatal("unsigned must not report stable")
	}
	if !Stable.Stable() {
		t.Fatal("stable must report stable")
	}
}
