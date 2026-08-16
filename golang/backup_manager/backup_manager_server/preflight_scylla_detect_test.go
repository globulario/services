package main

import (
	"os/exec"
	"strings"
	"testing"
)

// detectNativeScyllaDB returns the address scylla-manager will use to reach
// this node's manager-agent, so it must be routable. It probes the LAN IPs
// because that is where ScyllaDB's admin REST API binds (scylla.yaml
// api_address) — the API has remote consumers and is not on loopback.
//
// Two things are pinned here: never register a loopback host, and sweep the
// candidate addresses in a stable order so a multi-homed node does not
// register a different address run to run.

func TestScyllaDetectNeverRegistersLoopbackHost(t *testing.T) {
	var probed []string
	restore := stubExecCommand(func(name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if strings.HasPrefix(a, "http://") {
				probed = append(probed, a)
			}
		}
		return exec.Command("printf", "%s", "test-cluster")
	})
	defer restore()

	host, cluster := detectNativeScyllaDB()

	for _, u := range probed {
		if strings.Contains(u, "127.0.0.1") || strings.Contains(u, "//localhost") {
			t.Errorf("probed loopback %q — registering host=127.0.0.1 yields a cluster "+
				"scylla-manager cannot reach, which is worse than no registration", u)
		}
	}
	if host == "" {
		t.Skip("no routable LAN IP on this test host")
	}
	if strings.HasPrefix(host, "127.") {
		t.Fatalf("registration host = %q, must be routable", host)
	}
	if cluster != "test-cluster" {
		t.Errorf("cluster name = %q, want test-cluster", cluster)
	}
}

// The sweep must be deterministic: the same node yields the same registration
// address every run.
func TestScyllaDetectIsDeterministicAcrossRuns(t *testing.T) {
	restore := stubExecCommand(func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "%s", "test-cluster")
	})
	defer restore()

	first, _ := detectNativeScyllaDB()
	if first == "" {
		t.Skip("no routable LAN IP on this test host")
	}
	for i := 0; i < 8; i++ {
		got, _ := detectNativeScyllaDB()
		if got != first {
			t.Fatalf("registration host changed between runs: %q then %q — an unstable "+
				"address re-registers the node under a different host for no visible reason",
				first, got)
		}
	}
}

func TestScyllaDetectReturnsNothingWhenScyllaIsDown(t *testing.T) {
	restore := stubExecCommand(func(name string, args ...string) *exec.Cmd {
		return exec.Command("false") // curl -sf failure
	})
	defer restore()

	host, cluster := detectNativeScyllaDB()
	if host != "" || cluster != "" {
		t.Fatalf("expected no detection when the admin API does not answer, got host=%q cluster=%q",
			host, cluster)
	}
}

func stubExecCommand(f func(string, ...string) *exec.Cmd) func() {
	prev := execCommand
	execCommand = f
	return func() { execCommand = prev }
}
