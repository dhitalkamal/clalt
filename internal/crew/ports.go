package crew

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
)

// reserved says whether a port is never given out, even when a range covers
// it: 5476 is the port Kiro Crew uses by default, 5477 and 5478 are scratch
// gateways that somebody may run by hand, and 9100 to 9199 is where Kiro Crew's
// own app backends listen.
func reserved(p int) bool {
	return (p >= 5476 && p <= 5478) || (p >= 9100 && p <= 9199)
}

// AllocatePort returns the first port from first to last that is not reserved,
// not in taken (the ports of the other gateways, running or not) and not busy.
func AllocatePort(first, last int, taken map[int]bool, busy func(int) bool) (int, error) {
	for p := first; p <= last; p++ {
		if reserved(p) || taken[p] || (busy != nil && busy(p)) {
			continue
		}
		return p, nil
	}
	return 0, fmt.Errorf("no free port from %d to %d", first, last)
}

// PortBusy says whether something listens on the port of the loopback
// interface, on either address family: localhost can lead to both, and the
// gateway may bind either.
func PortBusy(port int) bool {
	return listenFails("127.0.0.1", port, func(error) bool { return true }) ||
		// no ipv6 on this machine says nothing about the port
		listenFails("::1", port, addrInUse)
}

func listenFails(host string, port int, counts func(error) bool) bool {
	l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return counts(err)
	}
	l.Close()
	return false
}

// addrInUse is a bind that failed because the port is taken. windows reports
// that as WSAEADDRINUSE (10048), which is not syscall.EADDRINUSE there.
func addrInUse(err error) bool {
	var errno syscall.Errno
	return errors.Is(err, syscall.EADDRINUSE) || (errors.As(err, &errno) && errno == 10048)
}
