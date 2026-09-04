package node_agentpb

// Probe status values carried in RunWorkflowResponse.Status for the synthetic
// probe-* workflows.
//
// These live beside the generated types because they are part of the same wire
// contract: the node agent writes them, the cluster controller reads them, and
// the two are separate binaries that can be at different versions. One
// canonical definition is what keeps "UNKNOWN" from being spelled
// independently on each side and drifting
// (identity.has_single_canonical_source_and_is_immutable).
const (
	// ProbeStatusSucceeded — the probe established that the component is
	// healthy.
	ProbeStatusSucceeded = "SUCCEEDED"

	// ProbeStatusFailed — the probe established that the component is NOT
	// healthy. This is a verdict, and callers may act on it, remediation
	// included.
	ProbeStatusFailed = "FAILED"

	// ProbeStatusUnknown — the probe ran but could not establish the
	// component's state: a reduced harvest, not a verdict.
	//
	// It exists because the alternative is worse than useless. In the 1.2.360
	// soak run a node agent whose own etcd client had lost its client
	// certificate reported its healthy local etcd as FAILED; the controller
	// raised infra_unhealthy on that member and remediation ground against it
	// for 55 reconcile cycles, because no remediation of etcd can repair an
	// agent's certificate. The agent knew it could not ask — it had no way to
	// say so.
	//
	// Consumers must treat this as "state unknown", never as healthy. A
	// consumer too old to recognise the value sees a non-SUCCEEDED status and
	// is no worse off than before.
	// See ops.always.doctor.reduced-harvest-honesty.
	ProbeStatusUnknown = "UNKNOWN"
)
