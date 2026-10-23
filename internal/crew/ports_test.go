package crew

import (
	"net"
	"strconv"
	"testing"
)

func never(int) bool { return false }

func TestTheFirstFreePortOfTheRangeIsChosen(t *testing.T) {
	got, err := AllocatePort(5480, 5490, nil, never)
	if err != nil || got != 5480 {
		t.Errorf("AllocatePort = %d, %v, want 5480", got, err)
	}
}

func TestPortsOfOtherGatewaysAndBusyPortsAreSkipped(t *testing.T) {
	taken := map[int]bool{5480: true, 5481: true}
	busy := func(p int) bool { return p == 5482 }
	got, err := AllocatePort(5480, 5490, taken, busy)
	if err != nil || got != 5483 {
		t.Errorf("AllocatePort = %d, %v, want 5483", got, err)
	}
}

func TestAppBackendPortsAreNeverGivenOutEvenWhenTheRangeCoversThem(t *testing.T) {
	for _, p := range []int{9100, 9150, 9199} {
		if got, err := AllocatePort(p, p, nil, never); err == nil {
			t.Errorf("port %d was given out as %d", p, got)
		}
	}
	if got, err := AllocatePort(9099, 9201, nil, never); err != nil || got != 9099 {
		t.Errorf("got %d, %v", got, err)
	}
	if got, err := AllocatePort(5476, 5479, nil, never); err != nil || got != 5479 {
		t.Errorf("5476 to 5478 are reserved, got %d, %v", got, err)
	}
}

func TestKiroCrewsOwnPortAndTheScratchGatewaysAreNeverGivenOut(t *testing.T) {
	for _, first := range []int{5470, 5476, 5477} {
		got, err := AllocatePort(first, 5481, nil, never)
		if err != nil {
			t.Fatal(err)
		}
		if got == 5476 || got == 5477 {
			t.Errorf("range starting at %d gave %d", first, got)
		}
		if got != 5470 && first == 5470 {
			t.Errorf("range starting at 5470 gave %d, want 5470", got)
		}
	}
}

func TestARangeWithNothingFreeIsAnErrorThatSaysSo(t *testing.T) {
	_, err := AllocatePort(5480, 5482, map[int]bool{5480: true, 5481: true, 5482: true}, never)
	if err == nil {
		t.Fatal("a full range gave a port")
	}
	if _, err := AllocatePort(5480, 5479, nil, never); err == nil {
		t.Error("an empty range gave a port")
	}
}

func TestPortBusyAnswersForARealListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if !PortBusy(port) {
		t.Errorf("port %d has a listener and was reported free", port)
	}
	l.Close()
	if PortBusy(port) {
		t.Errorf("port %d was released and is still reported busy", port)
	}
}

func TestPortBusySeesAListenerOnIPv6Only(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no ipv6 loopback: %v", err)
	}
	defer l.Close()
	_, p, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(p)
	if !PortBusy(port) {
		t.Errorf("port %d has an ipv6 listener and was reported free", port)
	}
}
