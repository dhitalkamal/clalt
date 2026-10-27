package registry

import "testing"

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestDetectTerminal(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Terminal
	}{
		{"nothing", nil, Terminal{Kind: "terminal"}},
		{"kitty", map[string]string{"KITTY_WINDOW_ID": "7", "KITTY_LISTEN_ON": "unix:/tmp/kitty-1"},
			Terminal{Kind: "kitty", KittyWindowID: "7", KittyListenOn: "unix:/tmp/kitty-1"}},
		{"kitty without a socket", map[string]string{"KITTY_WINDOW_ID": "3"}, Terminal{Kind: "kitty", KittyWindowID: "3"}},
		{"tmux", map[string]string{"TMUX": "/tmp/tmux-501/default,123,0", "TMUX_PANE": "%4"},
			Terminal{Kind: "tmux", TmuxSocket: "/tmp/tmux-501/default", TmuxPane: "%4"}},
		{"tmux wins over kitty, both facts kept", map[string]string{"TMUX": "/s,1,0", "TMUX_PANE": "%1", "KITTY_WINDOW_ID": "2"},
			Terminal{Kind: "tmux", TmuxSocket: "/s", TmuxPane: "%1", KittyWindowID: "2"}},
		{"tmux without a pane is not tmux", map[string]string{"TMUX": "/s,1,0"}, Terminal{Kind: "terminal"}},
		{"vscode", map[string]string{"TERM_PROGRAM": "vscode"}, Terminal{Kind: "ide", IDE: "vscode"}},
		{"cursor", map[string]string{"TERM_PROGRAM": "vscode", "CURSOR_TRACE_ID": "x"}, Terminal{Kind: "ide", IDE: "cursor"}},
		{"jetbrains", map[string]string{"TERMINAL_EMULATOR": "JetBrains-JediTerm"}, Terminal{Kind: "ide", IDE: "jetbrains"}},
		{"zed", map[string]string{"ZED_TERM": "true"}, Terminal{Kind: "ide", IDE: "zed"}},
		{"another terminal program", map[string]string{"TERM_PROGRAM": "iTerm.app"}, Terminal{Kind: "terminal"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectTerminal(env(c.env)); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
