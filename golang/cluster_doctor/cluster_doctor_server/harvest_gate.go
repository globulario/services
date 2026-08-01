package main

// harvest_gate.go decides, PER FINDING, whether a reduced-harvest snapshot
// should suppress auto-remediation.
//
// WHY THIS EXISTS
//
// The healer used to compare one boolean — snap.DataIncomplete — and downgrade
// the entire cycle from enforce to observe. The safety property behind that is
// real and must be preserved: a rule that sees empty data can emit a finding
// that would not exist with complete data, and auto-remediating a false positive
// is destructive (doctor.healer_auto_remediation_on_reduced_harvest).
//
// But DataIncomplete is set by ANY collector failure anywhere in the cluster,
// and the consequence was total. Observed across four consecutive bring-ups on
// 2026-08-01, each blocked by a different unrelated source:
//
//	node-3 Envoy unreachable          → cluster_controller RPCs failed
//	workflow → ScyllaDB degraded      → ListCorrelationDeferState failed
//	node-2 had no agent_endpoint      → node_agent dial failed
//	node-1 VerifyPackageIntegrity     → deadline exceeded
//
// In every case a drifted globular-* unit on a DIFFERENT, fully-harvested node
// went unrepaired, because a source that finding never reads was unhealthy. The
// healer stands down hardest exactly when the cluster is degraded — which is
// when it is most needed. On a healthy cluster there is nothing to repair; on a
// degraded one it refuses to act. That is the failure this file fixes.
//
// WHAT IS PRESERVED
//
// The rule is unchanged in substance: never auto-remediate a finding derived
// from data that could not be collected. It is only applied at the right
// granularity — the finding, rather than the cycle.
//
// An invariant with NO declared sources is never auto-remediated on a reduced
// harvest. Undeclared means unknown, and unknown means conservative: the gate
// cannot prove a finding is safe, so it does not.

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
)

// sourcesForInvariant declares which collector data sources a finding of a given
// invariant is derived from.
//
// An explicit table, for the same reason conditionForInvariant and
// draftForInvariant are: the mapping is a fact about how a rule computes its
// finding, and deriving it from a name or category would silently go wrong the
// moment either convention shifted — producing a gate that permits enforcement
// on data it never checked. That failure is invisible and destructive, so the
// mapping is written down and reviewed.
//
// Only invariants that can be auto-remediated need an entry. Everything else is
// classified propose/observe by policy and never reaches this gate; adding
// entries for them would be unreviewed guesses about rules nobody is about to
// let act.
var sourcesForInvariant = map[string][]string{
	// nodeUnitsRunning reads the node agent's unit inventory and nothing else.
	"node.systemd.units_running": {"node_agent"},

	// artifactIntegrity reads snap.IntegrityReports, populated per node from
	// node_agent VerifyPackageIntegrity (collector.go). Its findings carry
	// sub-ids; this is the one PolicyV1 marks HealAuto with a real AutoAction
	// (delete_stale_cache). Found by the parity test below rather than by
	// reading the policy — the healer file's comment still claims no HealAuto
	// rule has a non-empty AutoAction, which stopped being true.
	"artifact.cache_digest_mismatch": {"node_agent"},
}

// harvestGate answers "may this finding be auto-remediated given what failed to
// collect?" for one snapshot.
type harvestGate struct {
	incomplete bool
	// degraded maps a source service to the nodes it failed for. The empty node
	// key means the source failed cluster-wide (a collector that is not
	// per-node, e.g. cluster_controller or workflow).
	degraded map[string]map[string]bool
}

// newHarvestGate reads the snapshot's recorded failures.
//
// The collector records per-node failures as "node_agent@<nodeID>" and
// cluster-scoped ones bare. Splitting on "@" is not string-sniffing for meaning:
// it is reading back the exact encoding snapshot.addError writes.
func newHarvestGate(snap *collector.Snapshot) harvestGate {
	g := harvestGate{degraded: map[string]map[string]bool{}}
	if snap == nil {
		// No snapshot is maximally unknown, not maximally safe.
		g.incomplete = true
		return g
	}
	g.incomplete = snap.DataIncomplete
	for _, e := range snap.DataErrors {
		service, node := e.Service, ""
		if i := strings.IndexByte(service, '@'); i >= 0 {
			service, node = service[:i], service[i+1:]
		}
		if g.degraded[service] == nil {
			g.degraded[service] = map[string]bool{}
		}
		g.degraded[service][node] = true
	}
	return g
}

// enforceable reports whether f may be auto-remediated, and why not when it may
// not. The reason is returned rather than logged here so the caller can decide
// how loud to be — a per-finding suppression on a large cluster is common and
// must not become log spam.
func (g harvestGate) enforceable(f rules.Finding) (bool, string) {
	if !g.incomplete {
		return true, ""
	}

	// Incomplete with nothing attributable is the most dangerous shape: we know
	// data is missing and cannot say which. Not every path records a DataError —
	// collector.go sets DataIncomplete directly in at least one place — so this
	// is reachable in production, not just from a nil snapshot. Without an
	// attribution the per-source reasoning below has nothing to reason over, and
	// falling through it would silently permit exactly what this gate exists to
	// prevent.
	if len(g.degraded) == 0 {
		return false, "harvest incomplete with no attributable source; cannot determine " +
			"which findings are affected"
	}

	sources, declared := sourcesForInvariant[f.InvariantID]
	if !declared {
		return false, fmt.Sprintf("invariant %q declares no data sources; cannot prove the "+
			"finding survives a reduced harvest", f.InvariantID)
	}

	node := nodeOfEntity(f.EntityRef)
	for _, src := range sources {
		nodes, bad := g.degraded[src]
		if !bad {
			continue
		}
		if nodes[""] {
			return false, fmt.Sprintf("source %q failed cluster-wide", src)
		}
		if node == "" {
			// The finding names no node, so a per-node failure of a source it
			// depends on cannot be ruled out. Refuse rather than assume the
			// healthy nodes are the relevant ones.
			return false, fmt.Sprintf("source %q degraded on some nodes and finding %q names no node",
				src, f.FindingID)
		}
		if nodes[node] {
			return false, fmt.Sprintf("source %q degraded on node %s — this finding is derived from it",
				src, node)
		}
	}
	return true, ""
}

// nodeOfEntity extracts the node id from a finding's entity_ref.
//
// Node-scoped findings use "<nodeID>/<unit>". Anything without a separator is
// not node-scoped, and returning "" says exactly that rather than guessing —
// enforceable treats the unknown as a reason to refuse.
func nodeOfEntity(entityRef string) string {
	if i := strings.IndexByte(entityRef, '/'); i > 0 {
		return entityRef[:i]
	}
	return ""
}

// harvestSuppressedNotify reports one finding held back by reduced harvest.
//
// Info, not warning: on a large cluster a degraded source suppresses many
// findings at once, and a warning per finding would drown the one line that
// matters. It says WHICH source and WHY, so an operator can tell "the healer is
// broken" from "the healer declined to act on data it could not collect".
var harvestSuppressedNotify = func(f rules.Finding, why string) {
	logger.Info("healer: enforcement suppressed for this finding (reduced harvest)",
		"invariant_id", f.InvariantID,
		"entity_ref", f.EntityRef,
		"reason", why,
	)
}
