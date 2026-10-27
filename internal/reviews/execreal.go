package reviews

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RealExec runs programs for real.
type RealExec struct{}

// Run implements Exec. a program that exits non-zero returns what it wrote and
// an error that carries the first line of its stderr.
func (RealExec) Run(ctx context.Context, dir string, env []string, stdin string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	// a child that outlives the program and holds the pipes must not hold us
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if err == nil || errors.Is(err, exec.ErrWaitDelay) {
		return out.Bytes(), nil
	}
	if ctx.Err() != nil {
		return out.Bytes(), fmt.Errorf("%s was stopped: %w", name, ctx.Err())
	}
	line := strings.TrimSpace(strings.SplitN(errb.String(), "\n", 2)[0])
	if len(line) > 200 {
		line = strings.ToValidUTF8(line[:200], "")
	}
	return out.Bytes(), fmt.Errorf("%s failed: %v: %s", name, err, line)
}
