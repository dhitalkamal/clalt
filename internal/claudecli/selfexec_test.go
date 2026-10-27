package claudecli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// the fakes are small programs in internal/fakes. TestMain builds each one once
// per run, and a test gets its own copy under the name of the program it
// stands in for. a program builds the same way on every platform, which is what
// lets these tests run on windows where an sh script cannot.

// built is where TestMain put the fakes it built, by package name.
var built = map[string]string{}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// buildFakes compiles the named fakes into dir.
func buildFakes(dir string, names ...string) error {
	goBin := "go"
	if p, err := exec.LookPath("go"); err == nil {
		goBin = p
	} else {
		goBin = filepath.Join(runtime.GOROOT(), "bin", exeName("go"))
	}
	for _, name := range names {
		out := filepath.Join(dir, exeName(name))
		cmd := exec.Command(goBin, "build", "-o", out, "github.com/dhitalkamal/clalt/internal/fakes/"+name)
		if msg, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("building %s: %v\n%s", name, err, msg)
		}
		built[name] = out
	}
	return nil
}

// installFake puts a copy of a built fake at dir/as, by hard link when the
// volume allows it and by copy when not. a test must never write into the
// result: a link shares its file with every other test.
func installFake(fake, dir, as string) (string, error) {
	dst := filepath.Join(dir, exeName(as))
	if err := os.Link(built[fake], dst); err == nil {
		return dst, nil
	}
	src, err := os.Open(built[fake])
	if err != nil {
		return "", err
	}
	defer src.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return "", err
	}
	return dst, out.Close()
}

func mustInstallFake(t *testing.T, fake, dir, as string) string {
	t.Helper()
	p, err := installFake(fake, dir, as)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// exited runs a process that ends the way asked ("exit 3", "kill9") and
// returns the error exec gives for it, since an *exec.ExitError cannot be made
// by hand.
func exited(t *testing.T, how ...string) *exec.ExitError {
	t.Helper()
	bin := mustInstallFake(t, "fakeclaudecli", t.TempDir(), "claude-exit")
	err := exec.Command(bin, how...).Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("%v did not end with an exit error: %v", how, err)
	}
	return ee
}
