package rules

import (
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// awarenessGraphSeedEmpty fires when the awareness-graph service is reachable
// (the collector connected and queried it) but returned zero results. This
// happens when the Oxigraph RDF store was cleared after service startup —
// the embedded seed that runs at startup only fires on a fresh (zero-triple)
// store, so a runtime wipe leaves the service running-but-empty.
//
// Disposition: HealPropose. Restarting the awareness-graph service will
// re-trigger the startup seed from embedded NT data. A service restart is
// operator-initiated — the doctor proposes, does not execute.
type awarenessGraphSeedEmpty struct{}

func (awarenessGraphSeedEmpty) ID() string       { return "awareness_graph.seed_empty" }
func (awarenessGraphSeedEmpty) Category() string { return "ai" }
func (awarenessGraphSeedEmpty) Scope() string    { return "cluster" }

func (r awarenessGraphSeedEmpty) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Not reachable means the service is down or not registered — a different
	// rule (node.units.not_running) handles the down case; we only fire when
	// the service is up but the store is empty.
	if !snap.AwarenessGraphReachable {
		return nil
	}
	if !snap.AwarenessGraphQueryEmpty {
		return nil // graph has data — all good
	}
	return []Finding{newAwarenessGraphSeedEmptyFinding()}
}

func newAwarenessGraphSeedEmptyFinding() Finding {
	const id = "awareness_graph.seed_empty"
	return Finding{
		FindingID:       FindingID(id, "awareness-graph", "zero_triples"),
		InvariantID:     id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:        "ai",
		EntityRef:       "awareness-graph",
		Summary:         "awareness-graph is running but the RDF store contains no triples — AI briefing/impact/resolve will return empty results",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("awareness_graph", "Query(by_class,Invariant,limit=1)", map[string]string{
				"result_rows": "0",
				"note":        "startup seed runs only on an empty store; a runtime wipe requires a service restart",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Restart the awareness-graph service to re-trigger the embedded NT seed: "+
					"globular services restart awareness-graph",
				"globular services restart awareness-graph"),
			step(2,
				"Verify the graph is populated after restart: globular awareness query --mode by_class --class invariant --limit 5",
				"globular awareness query --mode by_class --class invariant --limit 5"),
		},
	}
}
