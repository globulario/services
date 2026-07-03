package main

import "testing"

func TestServiceNameFromBinary_KnownPlainBinaryNames(t *testing.T) {
	cases := []struct {
		bin  string
		want string
		ok   bool
	}{
		{"gateway", "gateway", true},
		{"xds", "xds", true},
		{"prometheus", "prometheus", true},
		{"node_exporter", "node-exporter", true},
		{"scylla_manager", "scylla-manager", true},
		{"scylla_manager_agent", "scylla-manager-agent", true},
	}
	for _, tc := range cases {
		got, ok := serviceNameFromBinary(tc.bin)
		if ok != tc.ok {
			t.Fatalf("serviceNameFromBinary(%q) ok=%v, want %v", tc.bin, ok, tc.ok)
		}
		if got != tc.want {
			t.Fatalf("serviceNameFromBinary(%q)=%q, want %q", tc.bin, got, tc.want)
		}
	}
}

func TestServiceNameFromBinary_ServerSuffixFallback(t *testing.T) {
	got, ok := serviceNameFromBinary("node_agent_server")
	if !ok {
		t.Fatal("serviceNameFromBinary(node_agent_server) should be recognized")
	}
	if got != "node-agent" {
		t.Fatalf("serviceNameFromBinary(node_agent_server)=%q, want node-agent", got)
	}
}
