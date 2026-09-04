package main

// Regression tests for runProbeEtcdHealth's verdict.
//
// The 1.2.360 soak run raised "infra_unhealthy on etcd@node-3" for 55
// consecutive reconcile cycles against a member that was healthy and serving
// the whole time. The agent's own etcd client had lost its client certificate,
// so every call it made timed out, and the probe converted "I could not ask"
// into "the component is down". Remediation then ground against etcd, which
// was never at fault and which no etcd remediation could have fixed.
//
// Two properties are pinned, both on the production classifier:
//
//  1. A probe that reached NO member — including its own — reports UNKNOWN,
//     not FAILED. The most likely broken party is the client.
//  2. An answering peer is never proof that THIS node's member is healthy
//     (infra.node_specific_truth_must_be_observed_via_node_local_client).

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestClassifyEtcdProbe_NoMemberReachable_IsUnknownNotFailed is the defect.
func TestClassifyEtcdProbe_NoMemberReachable_IsUnknownNotFailed(t *testing.T) {
	resp := classifyEtcdProbe(
		time.Now(),
		"https://10.10.0.13:2379",
		errors.New("context deadline exceeded"),
		0,
		[]string{"https://10.10.0.11:2379: context deadline exceeded"},
	)

	if resp.GetStatus() == "FAILED" {
		t.Fatal("an agent that reached no member, including its own, reported the " +
			"component FAILED — that is the 1.2.360 node-3 defect: the broken " +
			"party is the client, and no remediation of etcd can fix it")
	}
	if resp.GetStatus() != node_agentpb.ProbeStatusUnknown {
		t.Fatalf("want %s, got %s", node_agentpb.ProbeStatusUnknown, resp.GetStatus())
	}
	// The message has to point at the client, or the next investigation repeats
	// this one.
	detail := resp.GetError()
	if !strings.Contains(detail, "reached no member") ||
		!strings.Contains(detail, "certificate") {
		t.Fatalf("the detail must name the reduced harvest and where to look "+
			"next; got %q", detail)
	}
}

// TestClassifyEtcdProbe_LocalDownPeersUp_IsFailed is the control: the one case
// that genuinely earns FAILED must still produce it, or the fix would have
// traded a false positive for a blind spot.
func TestClassifyEtcdProbe_LocalDownPeersUp_IsFailed(t *testing.T) {
	resp := classifyEtcdProbe(
		time.Now(),
		"https://10.10.0.13:2379",
		errors.New("connection refused"),
		4,
		nil,
	)

	if resp.GetStatus() != "FAILED" {
		t.Fatalf("a local member that refuses connections while 4 peers answer is "+
			"a real local fault; want FAILED, got %s", resp.GetStatus())
	}
	if !strings.Contains(resp.GetError(), "while 4 peer(s) did") {
		t.Fatalf("the verdict must record what separated it from a reduced "+
			"harvest; got %q", resp.GetError())
	}
}

// TestClassifyEtcdProbe_PeerHealthIsNeverLocalHealth pins the invariant the
// old fallback broke. Its rule — "any reachable configured etcd endpoint
// implies etcd is healthy" — would report node-3 healthy because node-1
// answered. That false positive raises no drift, so nothing would ever catch
// it; only a test can.
func TestClassifyEtcdProbe_PeerHealthIsNeverLocalHealth(t *testing.T) {
	for _, peers := range []int{1, 4} {
		resp := classifyEtcdProbe(
			time.Now(),
			"https://10.10.0.13:2379",
			errors.New("connection refused"),
			peers,
			nil,
		)
		if resp.GetStatus() == "SUCCEEDED" {
			t.Fatalf("%d answering peer(s) were accepted as proof that THIS node's "+
				"member is healthy — infra.node_specific_truth_must_be_observed_via_node_local_client "+
				"requires per-node truth to come from the node's own member", peers)
		}
	}
}

// TestProbeUnknown_IsNotASuccessAndCarriesItsReason guards the helper: a
// reduced harvest must be distinguishable from success by any consumer — old
// or new — and must say what it could not establish.
func TestProbeUnknown_IsNotASuccessAndCarriesItsReason(t *testing.T) {
	resp := probeUnknown(time.Now(), "cannot determine this node's interface IP")

	if resp.GetStatus() == "SUCCEEDED" {
		t.Fatal("UNKNOWN must never read as SUCCEEDED — a consumer that does not " +
			"recognise the value has to fail closed, not treat it as healthy")
	}
	if resp.GetStatus() != node_agentpb.ProbeStatusUnknown {
		t.Fatalf("want %s, got %s", node_agentpb.ProbeStatusUnknown, resp.GetStatus())
	}
	if resp.GetError() == "" {
		t.Fatal("a reduced harvest that does not say what it could not establish " +
			"is the same dead end as the message it replaces")
	}
	if resp.GetStepsSucceeded() != 0 {
		t.Fatalf("UNKNOWN must not claim a succeeded step, got %d", resp.GetStepsSucceeded())
	}
}

// TestEtcdProbeBudgets_LocalAttemptCannotConsumeTheWhole pins the budget split.
//
// One shared 10s context used to cover the local attempt and all five fallback
// attempts. When the local call spent it, every peer call returned
// "context deadline exceeded" without a request being made, and the probe
// reported "etcd status failed on all endpoints" — five identical errors, four
// of which were never measured. That message sent the soak investigation to
// the wrong component for hours.
func TestEtcdProbeBudgets_LocalAttemptCannotConsumeTheWhole(t *testing.T) {
	if etcdHealthLocalAttemptBudget >= etcdHealthProbeBudget {
		t.Fatalf("the local attempt (%s) may not be allowed to consume the whole "+
			"probe budget (%s) — the peer questions must still be askable",
			etcdHealthLocalAttemptBudget, etcdHealthProbeBudget)
	}
	// Enough left for at least one peer question after a full local stall.
	remaining := etcdHealthProbeBudget - etcdHealthLocalAttemptBudget
	if remaining < etcdHealthPeerAttemptBudget {
		t.Fatalf("after a fully-stalled local attempt only %s remains, which is "+
			"less than one peer attempt (%s): the peers would again be reported "+
			"as failures that were never measured",
			remaining, etcdHealthPeerAttemptBudget)
	}
}
