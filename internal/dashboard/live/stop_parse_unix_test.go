//go:build !windows

package live

import (
	"testing"
	"time"
)

func TestParsePS(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.ParseInLocation(psStart, s, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		name, in string
		want     Proc
		bad      bool
	}{
		{"macos", "Thu Oct 23 15:05:12 2026  4242 /opt/bin/claude --resume\n", Proc{true, at("Thu Oct 23 15:05:12 2026"), 4242, "/opt/bin/claude --resume"}, false},
		{"linux procps", "Thu Oct 23 15:05:12 2026     1 /sbin/init splash\n", Proc{true, at("Thu Oct 23 15:05:12 2026"), 1, "/sbin/init splash"}, false},
		{"day padded", "Fri Jan  2 03:04:05 2026 77 claude\n", Proc{true, at("Fri Jan  2 03:04:05 2026"), 77, "claude"}, false},
		{"kernel thread", "Thu Oct 23 15:05:12 2026     2 [kthreadd]\n", Proc{true, at("Thu Oct 23 15:05:12 2026"), 2, "[kthreadd]"}, false},
		{"defunct", "Thu Oct 23 15:05:12 2026  4242 [claude] <defunct>\n", Proc{}, false},
		{"empty", "", Proc{}, true},
		{"no command", "Thu Oct 23 15:05:12 2026 4242\n", Proc{}, true},
		{"bad month", "Thu Foo 23 15:05:12 2026 4242 claude\n", Proc{}, true},
		{"bad ppid", "Thu Oct 23 15:05:12 2026 x42 claude\n", Proc{}, true},
		{"text", "ps: illegal option\n", Proc{}, true},
	}
	for _, c := range cases {
		got, err := parsePS(1, c.in)
		if (err != nil) != c.bad || got != c.want {
			t.Errorf("%s: got %+v err %v, want %+v bad=%v", c.name, got, err, c.want, c.bad)
		}
	}
}
