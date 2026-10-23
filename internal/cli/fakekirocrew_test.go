package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// until a test installs the fake, kirocrew is a path that does not exist, so a
// test that forgot cannot reach the real one.
func init() { os.Setenv("CLALT_KIROCREW_BIN", "/nonexistent/clalt-test-kirocrew") }

type fakeKirocrew struct {
	t   *testing.T
	dir string
}

// installFakeKirocrew makes the fake the kirocrew of the test and lets all
// gateways go when the test ends.
func installFakeKirocrew(t *testing.T) *fakeKirocrew {
	t.Helper()
	dir := t.TempDir()
	bin := mustInstallFake(t, "fakekirocrew", dir, "kirocrew")
	t.Setenv("CLALT_KIROCREW_BIN", bin)
	t.Setenv("FAKE_KIROCREW_VERSION", "")
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release"), nil, 0o644)
	})
	return &fakeKirocrew{t: t, dir: dir}
}

func (f *fakeKirocrew) calls() []string {
	b, _ := os.ReadFile(filepath.Join(f.dir, "calls"))
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func (f *fakeKirocrew) gatewayEnv() map[string]string {
	// the fake is detached and Ready is stubbed, so the file can still be
	// empty or half written when start returns: wait for it to fill
	var b []byte
	var err error
	for i := 0; i < 100; i++ {
		b, err = os.ReadFile(filepath.Join(f.dir, "gateway.env"))
		if err == nil && strings.Contains(string(b), "KIROCREW_PORT=") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		f.t.Fatalf("no gateway was started: %v", err)
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env
}

// crewEnv is a clalt install with a signed-in account called work and the fake kirocrew.
func crewEnv(t *testing.T) (*testEnv, *fakeKirocrew) {
	t.Helper()
	skipWindows(t, "clalt crew is not available on windows")
	e := newEnv(t)
	e.seedShared()
	k := installFakeKirocrew(t)
	// every gateway looks ready at once, and the fake has no sockets to inspect
	e.app.crewOpts.Ready = func(context.Context, int) bool { return true }
	e.app.crewOpts.Listeners = func(context.Context, int) ([]string, error) { return []string{"127.0.0.1:5500"}, nil }
	// no real port is ever probed, so a gateway on this machine cannot shift the port
	e.app.crewOpts.Busy = func(int) bool { return false }
	e.signIn("work", "w@example.com")
	return e, k
}

// stopFake lets every gateway of the fake end by itself.
func stopFake(t *testing.T, k *fakeKirocrew) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(k.dir, "release"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}
