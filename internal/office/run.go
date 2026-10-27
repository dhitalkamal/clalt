package office

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// Options is how the office runs. Port 0 picks a free one. Open is
// OpenBrowser when nil and is never called with NoOpen. Out gets the URL.
type Options struct {
	Port     int
	NoOpen   bool
	Open     func(url string) error
	Out      io.Writer
	Snapshot func() (Snapshot, error)
	Logf     func(format string, args ...any)
	Layouts  *layout.Store
	// Generator builds the garden templates, Template is the one shown first,
	// see Config.
	Generator Generator
	Template  string
	// Activity finds transcripts for GET /api/activity, see Config.
	Activity ActivitySource
}

// Instance is a running server.
type Instance struct {
	Addr  net.Addr
	URL   string // with the token, for the first load
	Token string
	srv   *http.Server
	done  chan error
}

// newToken is 128 random bits as hex.
func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Start listens on 127.0.0.1 and serves in the background. the address is
// never anything but loopback.
func Start(opt Options) (*Instance, error) {
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", opt.Port))
	if err != nil {
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	h, err := NewHandler(Config{
		Token: tok, Port: port, Hosts: []string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("localhost:%d", port)},
		Snapshot: opt.Snapshot, Logf: opt.Logf, Layouts: opt.Layouts,
		Generator: opt.Generator, Template: opt.Template, Activity: opt.Activity,
	})
	if err != nil {
		ln.Close()
		return nil, err
	}
	inst := &Instance{
		Addr: ln.Addr(), Token: tok, URL: fmt.Sprintf("http://127.0.0.1:%d/?t=%s", port, tok), done: make(chan error, 1),
		srv: &http.Server{
			Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
			WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second,
		},
	}
	go func() {
		err := inst.srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		inst.done <- err
	}()
	return inst, nil
}

// Close stops the server, letting requests in flight finish for a moment.
func (i *Instance) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := i.srv.Shutdown(ctx); err != nil {
		_ = i.srv.Close()
	}
	return <-i.done
}

// Run starts the office, prints the URL, opens the browser unless NoOpen and
// serves until ctx ends. a browser that cannot be opened is only a warning,
// the URL is already printed.
func Run(ctx context.Context, opt Options) error {
	inst, err := Start(opt)
	if err != nil {
		return err
	}
	out := opt.Out
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintf(out, "clalt office is running on this machine only. open:\n\n  %s\n\nthe link carries a token made for this run. press Ctrl-C to stop.\n", inst.URL)
	if !opt.NoOpen {
		open := opt.Open
		if open == nil {
			open = OpenBrowser
		}
		if err := open(inst.URL); err != nil {
			fmt.Fprintf(out, "could not open a browser (%v), open the link above yourself.\n", err)
		}
	}
	select {
	case <-ctx.Done():
		return inst.Close()
	case err := <-inst.done:
		return err
	}
}
