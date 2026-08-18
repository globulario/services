package main

import (
	"strings"
	"testing"
)

// ScyllaDB's admin REST API (port 10000) binds the node's routable address
// (scylla.yaml api_address), because it has remote consumers: the controller
// calls /storage_service/host_id and /storage_service/remove_node on OTHER
// nodes during ring removal, and backup_manager registers that address with
// scylla-manager.
//
// Local consumers (nodetool, the join gate, the health probe) must name that
// address explicitly — nodetool's own default is loopback, so leaving it
// implicit points it at nothing. The address must also be chosen the SAME way
// every run; picking it out of a Go map made a multi-homed node probe a
// different address each time and fail intermittently.

func TestScyllaAdminAPIHostIsDeterministic(t *testing.T) {
	ips := map[string]bool{
		"10.10.0.12":  true,
		"172.17.0.1":  true,
		"192.168.1.5": true,
		"127.0.0.1":   true,
	}
	first := scyllaAdminAPIHost(ips)
	if first == "" {
		t.Fatal("no admin API host chosen from a set containing routable IPs")
	}
	for i := 0; i < 50; i++ {
		if got := scyllaAdminAPIHost(ips); got != first {
			t.Fatalf("admin API host varied across calls: %q then %q — Go map iteration "+
				"order is randomised, so an unsorted pick makes nodetool probe a "+
				"different address each run", first, got)
		}
	}
}

func TestScyllaAdminAPIHostExcludesLoopback(t *testing.T) {
	got := scyllaAdminAPIHost(map[string]bool{"10.10.0.12": true, "127.0.0.1": true})
	if strings.HasPrefix(got, "127.") {
		t.Fatalf("chose loopback %q — api_address is the routable IP, so nodetool "+
			"would connect to nothing and report the node unobservable", got)
	}
	if got != "10.10.0.12" {
		t.Fatalf("admin API host = %q, want 10.10.0.12", got)
	}
}

// With only loopback available there is no routable address to name, so the
// probe falls back to nodetool's default rather than passing a bad -h.
func TestScyllaAdminAPIHostEmptyWhenOnlyLoopback(t *testing.T) {
	if got := scyllaAdminAPIHost(map[string]bool{"127.0.0.1": true}); got != "" {
		t.Fatalf("expected no host when only loopback is present, got %q", got)
	}
}

// The manager-agent reaches Scylla at the same api_address, and its own listen
// ports stay on the node IP because scylla-manager connects from off-node.
func TestAgentConfigUsesNodeIPForBothScyllaAPIAndListenPorts(t *testing.T) {
	const nodeIP = "10.10.0.12"

	cfg := upsertScyllaAPIURL("auth_token: abc\n", nodeIP)
	if !hasScyllaAPIURL(cfg, nodeIP) {
		t.Fatalf("agent does not point at the node's admin API address:\n%s", cfg)
	}
	if !strings.Contains(cfg, "auth_token: abc") {
		t.Fatalf("address reconcile dropped the cluster-wide auth token:\n%s", cfg)
	}

	cfg = upsertScyllaAgentPorts(cfg, nodeIP)
	if !hasScyllaAgentPorts(cfg, nodeIP) {
		t.Fatalf("agent listen ports are not bound to the node IP %s:\n%s", nodeIP, cfg)
	}
}
