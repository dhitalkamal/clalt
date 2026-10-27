package office

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openerCommand is the command that opens url in the default browser on goos.
func openerCommand(goos, url string) ([]string, error) {
	switch goos {
	case "darwin":
		return []string{"open", url}, nil
	case "linux":
		return []string{"xdg-open", url}, nil
	case "windows":
		// the empty string is the window title that start would otherwise take from a quoted url
		return []string{"cmd", "/c", "start", "", url}, nil
	}
	return nil, fmt.Errorf("no way to open a browser on %s", goos)
}

// OpenBrowser opens url in the default browser. it does not wait for it.
func OpenBrowser(url string) error {
	argv, err := openerCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
