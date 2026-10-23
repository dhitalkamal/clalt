package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/fakes/fakeutil"
)

// fakeKirocrewMain is what this binary does when it is started as kirocrew. a
// gateway is a process that records how it was started, prints a line with a
// token in it, and holds until a release file appears (or a minute has passed).
func main() {
	os.Exit(fakeKirocrewMain(os.Args[1:]))
}

func fakeKirocrewMain(args []string) int {
	dir := fakeutil.Beside()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "fake kirocrew: unexpected")
		return 64
	}
	switch args[0] {
	case "--version":
		v := os.Getenv("FAKE_KIROCREW_VERSION")
		if v == "" {
			v = "0.9.0"
		}
		fmt.Println("kirocrew " + v)
		return 0
	case "config":
		fakeutil.AppendLine(filepath.Join(dir, "calls"), "config "+strings.Join(args, " "))
		return 0
	case "gateway":
		fakeutil.AppendLine(filepath.Join(dir, "calls"), "gateway "+strings.Join(args, " "))
		fakeutil.WriteEnv(filepath.Join(dir, "gateway.env"))
		fmt.Println("gateway up, open http://localhost/?token=abc123secret")
		for i := 0; i < 1200; i++ {
			if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "fake kirocrew: unexpected "+strings.Join(args, " "))
	return 64
}
