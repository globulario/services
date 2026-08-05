package oci

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCheckProbeHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	host, portText, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	probe := Probe{Type: "http", Address: host, Port: uint16(port), Path: "/ready", ExpectedStatusMin: 200, ExpectedStatusMax: 299}
	if err := CheckProbe(context.Background(), probe); err != nil {
		t.Fatal(err)
	}
}

func TestWaitProbeIsBoundedByFailureThreshold(t *testing.T) {
	probe := Probe{Type: "tcp", Address: "127.0.0.1", Port: 1, TimeoutMillis: 10, IntervalMillis: 1, FailureThreshold: 2}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	if err := WaitProbe(ctx, probe); err == nil {
		t.Fatal("WaitProbe() succeeded against closed port")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("WaitProbe() exceeded bounded test duration")
	}
}
