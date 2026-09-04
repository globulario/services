package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Wiping /var/lib/globular/etcd/member turns this node into an UNINITIALIZED
// etcd member, and an uninitialized member takes its entire membership from
// initial-cluster in etcd.yaml. Nothing refreshes that file between membership
// changes (dispatchPlan, which used to carry the controller's render to the
// node, is a no-op stub), so the wipe converts a stale file into a permanent
// failure: etcd exits 1 with "error validating peerURLs ...: member count is
// unequal" on every start and never converges.
//
// Measured on the 5-node simulation, 1.2.359, 2026-09-03: node-5's etcd.yaml
// listed 4 members while the ring held 5; wipe-etcd-and-rejoin ran at 15:30:23,
// 15:33:23, 15:36:23 and 15:39:23 and reported SUCCEEDED each time; the scenario
// authority/node-clone-identity-collision failed its restoration postcondition
// with etcd_healthy_endpoints baseline=5 now=4.
//
// The contract this file has always carried —
// intent.node_agent.destructive_member_wipes_require_controller_prep — is now
// checked instead of assumed.
func TestRunWipeEtcdAndRejoin_RefusesWithoutRenderedConfig(t *testing.T) {
	srv := &NodeAgentServer{}

	resp, err := srv.runWipeEtcdAndRejoin(context.Background(), &node_agentpb.RunWorkflowRequest{
		WorkflowName: "wipe-etcd-and-rejoin",
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetStatus() != "FAILED" {
		t.Fatalf("status = %q, want FAILED — a wipe without current membership must be refused, not attempted",
			resp.GetStatus())
	}
	if !strings.Contains(resp.GetError(), etcdRejoinConfigInput) {
		t.Errorf("error %q does not name the missing input %q — the operator cannot act on it",
			resp.GetError(), etcdRejoinConfigInput)
	}
	if resp.GetStepsSucceeded() != 0 {
		t.Errorf("steps_succeeded = %d, want 0 — nothing may run before the guard passes", resp.GetStepsSucceeded())
	}
}

// An empty inputs map and a missing key must be refused identically: the guard
// is about having the membership, not about the shape of the request.
func TestRunWipeEtcdAndRejoin_EmptyInputsAreRefusedToo(t *testing.T) {
	srv := &NodeAgentServer{}
	for name, req := range map[string]*node_agentpb.RunWorkflowRequest{
		"nil inputs":  {},
		"empty map":   {Inputs: map[string]string{}},
		"empty value": {Inputs: map[string]string{etcdRejoinConfigInput: ""}},
		"wrong key":   {Inputs: map[string]string{"etcd_yaml": "name: node-5"}},
	} {
		resp, err := srv.runWipeEtcdAndRejoin(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: unexpected transport error: %v", name, err)
		}
		if resp.GetStatus() != "FAILED" {
			t.Errorf("%s: status = %q, want FAILED", name, resp.GetStatus())
		}
	}
}

// The repair rewrites the membership and NOTHING else. The installer and the
// controller's renderer disagree about the rest of etcd.yaml — the installer
// writes client-cert-auth: true on both transports, the renderer writes neither —
// so replacing the whole file during a repair would silently drop client
// certificate authentication on the repaired etcd endpoint.
func TestRewriteEtcdInitialCluster_ChangesOnlyTheMembership(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "etcd.yaml")
	const original = `name: node-5
data-dir: /var/lib/globular/etcd

initial-advertise-peer-urls: https://10.10.0.15:2380
listen-peer-urls: https://0.0.0.0:2380

initial-cluster: node-4=https://10.10.0.14:2380,node-3=https://10.10.0.13:2380
initial-cluster-state: "existing"
initial-cluster-token: "globular-etcd-cluster"

client-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  trusted-ca-file: /var/lib/globular/pki/ca.crt
  client-cert-auth: true
`
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const ring = "node-4=https://10.10.0.14:2380,node-2=https://10.10.0.12:2380,node-3=https://10.10.0.13:2380,node-5=https://10.10.0.15:2380,globular-etcd=https://10.10.0.11:2380"
	if err := rewriteEtcdInitialCluster(path, ring); err != nil {
		t.Fatalf("rewriteEtcdInitialCluster: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	out := string(got)

	if !strings.Contains(out, `initial-cluster: "`+ring+`"`) {
		t.Errorf("membership not written; got:\n%s", out)
	}
	for _, keep := range []string{
		`initial-cluster-state: "existing"`,
		`initial-cluster-token: "globular-etcd-cluster"`,
		"client-cert-auth: true",
		"trusted-ca-file: /var/lib/globular/pki/ca.crt",
		"listen-peer-urls: https://0.0.0.0:2380",
		"name: node-5",
	} {
		if !strings.Contains(out, keep) {
			t.Errorf("repair removed %q — it may only change initial-cluster", keep)
		}
	}
	if strings.Contains(out, "node-4=https://10.10.0.14:2380,node-3=https://10.10.0.13:2380\n") {
		t.Error("the stale membership survived")
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640", st.Mode().Perm())
	}

	// No temp file may survive: a leftover .etcd.yaml.* beside the real file is
	// indistinguishable from a partial write to anything that scans the dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".etcd.yaml.") {
			t.Errorf("temp file %s left behind", e.Name())
		}
	}
}

// A config with no initial-cluster key, or no file at all, means the repair
// cannot know what this node must come back with. It must fail rather than
// invent one: a node that rejoins with guessed membership is the failure this
// whole path exists to prevent.
func TestRewriteEtcdInitialCluster_RefusesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "absent.yaml")
	if err := rewriteEtcdInitialCluster(missing, "a=https://10.0.0.1:2380"); err == nil {
		t.Error("a missing etcd.yaml was accepted — the repair must refuse")
	}

	noKey := filepath.Join(dir, "nokey.yaml")
	if err := os.WriteFile(noKey, []byte("name: node-5\ndata-dir: /var/lib/globular/etcd\n"), 0o640); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := rewriteEtcdInitialCluster(noKey, "a=https://10.0.0.1:2380"); err == nil {
		t.Error("an etcd.yaml with no initial-cluster key was accepted — the repair must refuse")
	}
}
