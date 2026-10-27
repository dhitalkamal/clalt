package registry

import "strings"

// DetectTerminal reads the terminal location out of the environment, through
// getenv so a test can hand it a map. it is best effort: the IDE markers are the
// ones the common terminals set, and a miss only means kind "terminal".
func DetectTerminal(getenv func(string) string) Terminal {
	t := Terminal{
		KittyWindowID: getenv("KITTY_WINDOW_ID"),
		KittyListenOn: getenv("KITTY_LISTEN_ON"),
	}
	pane := getenv("TMUX_PANE")
	if tmux := getenv("TMUX"); tmux != "" && pane != "" {
		t.TmuxSocket, _, _ = strings.Cut(tmux, ",")
		t.TmuxPane = pane
	}
	switch ide := detectIDE(getenv); {
	case t.TmuxPane != "":
		t.Kind = "tmux"
	case t.KittyWindowID != "":
		t.Kind = "kitty"
	case ide != "":
		t.Kind, t.IDE = "ide", ide
	default:
		t.Kind = "terminal"
	}
	return t
}

func detectIDE(getenv func(string) string) string {
	switch {
	case getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm":
		return "jetbrains"
	case getenv("ZED_TERM") != "":
		return "zed"
	case getenv("TERM_PROGRAM") == "vscode" && getenv("CURSOR_TRACE_ID") != "":
		return "cursor"
	case getenv("TERM_PROGRAM") == "vscode":
		return "vscode"
	}
	return ""
}
