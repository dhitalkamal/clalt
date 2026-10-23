package crew

import (
	"context"
	"net"
	"testing"
)

func TestDialReadyIsTrueOnlyWhileSomethingListens(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if !DialReady(context.Background(), port) {
		t.Errorf("port %d has a listener and was not ready", port)
	}
	l.Close()
	if DialReady(context.Background(), port) {
		t.Errorf("port %d has no listener and was ready", port)
	}
}

func TestDialReadyStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if DialReady(ctx, 5480) {
		t.Error("a cancelled context was ready")
	}
}
