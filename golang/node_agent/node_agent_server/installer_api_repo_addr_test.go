package main

import "testing"

func TestIsLocalEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		localIP string
		want    bool
	}{
		{name: "localhost", addr: "localhost:443", localIP: "10.0.0.20", want: true},
		{name: "loopback", addr: "127.0.0.1:443", localIP: "10.0.0.20", want: true},
		{name: "local ip", addr: "10.0.0.20:443", localIP: "10.0.0.20", want: true},
		{name: "remote ip", addr: "10.0.0.63:443", localIP: "10.0.0.20", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalEndpoint(tt.addr, tt.localIP); got != tt.want {
				t.Fatalf("isLocalEndpoint(%q,%q)=%v want %v", tt.addr, tt.localIP, got, tt.want)
			}
		})
	}
}

