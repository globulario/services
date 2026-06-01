package main

// EtcdMemberIntent is the controller-authorized etcd membership state for a node.
//
// Design rule: the controller writes this; no other component may promote a
// node to etcd voter without an explicit intent record.
//
// "control-plane" profile is a capability label — it does NOT automatically
// make the node an etcd voter. The controller must set Member=true and
// Voter=true explicitly.
type EtcdMemberIntent struct {
	// Member is true when the controller has authorized this node to be an etcd
	// cluster member. False means the node must not attempt an etcd join.
	Member bool `json:"member"`
	// Voter is true when the controller has authorized this node to be a full
	// voting member (not a learner). Requires Member=true.
	Voter bool `json:"voter"`
	// Generation is the controller state generation at which this intent was
	// written. Used to detect stale intents after leader changes.
	Generation uint64 `json:"generation"`
	// Reason is a human-readable explanation for the current intent state.
	Reason string `json:"reason,omitempty"`
}

// ScyllaIntent is the controller-authorized ScyllaDB membership state for a node.
//
// Design rule: "storage" profile is a capability label — it does NOT
// automatically make the node an RF contributor. The controller must explicitly
// set Member=true and, only after runtime proof, RFEligible=true.
//
// RFEligible=false must be the default for any newly joining node. The
// controller advances it to true only after ScyllaDB has joined the gossip
// ring and the node's runtime health passes the Group 0 checks.
type ScyllaIntent struct {
	// Member is true when the controller has authorized this node to join the
	// ScyllaDB cluster. False means the node must not start scylla-server.
	Member bool `json:"member"`
	// RFEligible is true when this node may be counted toward the replication
	// factor used by keyspace RF policy and the schema guard. Must be false
	// until both admission and runtime proof (ScyllaJoinVerified) exist.
	//
	// INVARIANT: RFEligible must not be set true during JoinPlan issuance.
	// It requires a separate runtime-proof confirmation step.
	RFEligible bool `json:"rf_eligible"`
	// Group0VoterVerified is true when the controller has confirmed that this
	// node is a live, reachable Group 0 voter in the Scylla Raft schema cluster.
	// Only meaningful when Member=true.
	Group0VoterVerified bool `json:"group0_voter_verified,omitempty"`
	// Generation is the controller state generation at which this intent was
	// written.
	Generation uint64 `json:"generation"`
	// Reason is a human-readable explanation for the current intent state.
	Reason string `json:"reason,omitempty"`
}

// ObjectStoreIntent is the controller-authorized MinIO pool membership state
// for a node.
//
// Design rule: "storage" profile is a capability label — it does NOT
// automatically make the node a MinIO pool member. The controller must set
// Member=true explicitly and the MinIO topology system must confirm the join.
//
// Note: ObjectStoreIntent does not affect Scylla RF eligibility. It is
// tracked separately because MinIO and ScyllaDB are independent primitives.
type ObjectStoreIntent struct {
	// Member is true when the controller has authorized this node to join the
	// MinIO erasure-coded pool.
	Member bool `json:"member"`
	// TopologyGeneration is the MinIO topology generation at which this node
	// was admitted to the pool. Zero means the node has not been admitted to
	// any topology generation.
	TopologyGeneration uint64 `json:"topology_generation"`
	// Reason is a human-readable explanation for the current intent state.
	Reason string `json:"reason,omitempty"`
}

// initialScyllaIntentForProfiles returns the default ScyllaIntent for a newly
// authorized node given its assigned profiles.
//
// Key invariant: RFEligible is always false for a newly joining node — even
// when the "storage" profile is assigned. The controller must not grant RF
// eligibility until runtime proof exists (ScyllaJoinVerified + Group 0 health).
func initialScyllaIntentForProfiles(profiles []string) *ScyllaIntent {
	member := false
	for _, p := range profiles {
		if p == "storage" || p == "control-plane" {
			member = true
			break
		}
	}
	if !member {
		return nil
	}
	return &ScyllaIntent{
		Member:     true,
		RFEligible: false, // intentionally false: requires runtime proof
		Reason:     "storage profile assigned; rf_eligible=false until runtime proof",
	}
}

// initialEtcdIntentForProfiles returns the default EtcdMemberIntent for a
// newly authorized node given its assigned profiles.
//
// The controller sets Voter=false by default. The etcd join state machine
// advances it to Voter=true once the join succeeds.
func initialEtcdIntentForProfiles(profiles []string) *EtcdMemberIntent {
	member := false
	for _, p := range profiles {
		if p == "control-plane" || p == "core" {
			member = true
			break
		}
	}
	if !member {
		return nil
	}
	return &EtcdMemberIntent{
		Member: true,
		Voter:  false, // intentionally false: requires etcd join completion
		Reason: "control-plane/core profile assigned; voter=false until join completes",
	}
}

// initialObjectStoreIntentForProfiles returns the default ObjectStoreIntent
// for a newly authorized node given its assigned profiles.
func initialObjectStoreIntentForProfiles(profiles []string) *ObjectStoreIntent {
	for _, p := range profiles {
		if p == "storage" {
			return &ObjectStoreIntent{
				Member: true,
				Reason: "storage profile assigned; topology_generation=0 until MinIO pool join",
			}
		}
	}
	return nil
}
