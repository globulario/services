package ports

import (
	"net"
	"testing"
)

func TestPortFreeDetectsWildcardInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Skipf("cannot bind wildcard listener: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if portFree(port) {
		t.Fatalf("expected portFree to be false for wildcard listener on port %d", port)
	}
}

func TestReserveSkipsInfraReservedPorts(t *testing.T) {
	alloc := &Allocator{start: 10000, end: 10005, reserved: make(map[int]string)}
	port, err := alloc.Reserve("svc")
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if _, blocked := infraReservedPorts[port]; blocked {
		t.Fatalf("allocator returned reserved infra port %d", port)
	}
	if port != 10001 {
		t.Fatalf("expected first non-reserved port 10001, got %d", port)
	}
}
