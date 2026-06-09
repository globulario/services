// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.invariant_registry_source_error_enforcement_test
// @awareness file_role=ratchet_gate_no_rule_emits_confident_finding_on_errored_snapshot
// @awareness enforces=globular.platform:invariant.doctor_rule_evaluate_must_consult_snap_errors
// @awareness risk=high
package rules

import (
	"errors"
	"sort"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// errSourceDown is the sentinel sub-fetch error used by fullyErroredSnapshot.
var errSourceDown = errors.New("collector sub-fetch failed (test)")

// fullyErroredSnapshot models "the collector got nothing this sweep": every
// typed field is empty/nil, DataIncomplete=true, and EVERY per-source error
// signal the rules consult is set — not just DataErrors, but the dedicated
// fields (CriticalKeyQueryError, IngressSpecLoadError, ObjectStoreDesiredLoadError)
// that specific rules use to tell "source errored" from "confirmed absent".
//
// Under this snapshot a source-consuming rule must NOT emit a confident
// (INVARIANT_FAIL) finding — absence here is "unknown", not "confirmed". A FAIL
// means the rule treated an errored source as a definitive negative (the
// FALSE_POSITIVE half of the masking bug class). The registry's
// snapshotSourceUnavailableFindings and per-key checkErrorFinding carry a
// non-empty CheckError, so they are never counted as confident.
func fullyErroredSnapshot() *collector.Snapshot {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "cluster_controller", RPC: "ListNodes", Err: errSourceDown},
			{Service: "cluster_controller", RPC: "GetClusterHealthV1", Err: errSourceDown},
			{Service: "cluster_controller", RPC: "GetDesiredState", Err: errSourceDown},
			{Service: "cluster_controller", RPC: "ListDesiredBuildIDs", Err: errSourceDown},
			{Service: "etcd", RPC: "Get(/globular/controller/leader_pending_update)", Err: errSourceDown},
			{Service: "etcd", RPC: "Get(/globular/ingress/v1/status/)", Err: errSourceDown},
			{Service: "etcd", RPC: "Get(/globular/scylla/schema_guard/)", Err: errSourceDown},
			{Service: "etcd", RPC: "LoadObjectStoreDesiredState", Err: errSourceDown},
			{Service: "repository", RPC: "GetRepositoryStatus", Err: errSourceDown},
			{Service: "repository", RPC: "ListRepositoryFindings", Err: errSourceDown},
			{Service: "repository", RPC: "ListArtifacts", Err: errSourceDown},
		},
		IngressSpecLoadError:        errSourceDown,
		ObjectStoreDesiredLoadError: errSourceDown,
		CriticalKeyQueryError:       map[string]error{},
	}
	// Rules that read critical etcd keys consult CriticalKeyQueryError per key;
	// mark every critical key/prefix as "query failed" so the rule emits
	// CHECK_ERROR (indeterminate), not a confident "key absent" FAIL.
	for _, k := range config.CriticalEtcdKeys {
		snap.CriticalKeyQueryError[k] = errSourceDown
	}
	for _, p := range config.CriticalEtcdPrefixes {
		snap.CriticalKeyQueryError[p] = errSourceDown
	}
	return snap
}

func isConfidentFailure(f Finding) bool {
	if f.CheckError != "" {
		return false // explicitly indeterminate — never a confident verdict
	}
	return f.InvariantStatus == cluster_doctorpb.InvariantStatus_INVARIANT_FAIL
}

// knownSourceIndependentConfidentRules allowlists rules that legitimately emit
// a confident finding on a fully-errored RPC snapshot because their data source
// is NOT an errored RPC sub-fetch. This map may only SHRINK: check (2) below
// fails if an entry stops emitting (forcing its removal), and a NEW rule that
// emits a confident FAIL on errored data is rejected by check (1) until it is
// either guarded with snap.HadError or added here with a justification and
// review. That is the ratchet that keeps the masking bug class from returning.
var knownSourceIndependentConfidentRules = map[string]string{
	// Reads the local filesystem (os.ReadDir/Stat of the artifact-state root);
	// its confident findings are local-disk drift observations, not derived from
	// an errored RPC source. Follow-up: its secondary use of snap.Inventories
	// could be guarded so it does not over-report unknown dirs when the
	// node-agent inventory RPC errored.
	"artifact.layout_drift_local": "local-filesystem check; confident findings come from local disk, not an errored RPC source",
}

// TestNoRuleEmitsConfidentFailureOnErroredSnapshot is the enforcement ratchet
// for the doctor masking bug class (FALSE_POSITIVE half): no registered rule may
// emit a confident INVARIANT_FAIL when the collector snapshot is fully errored,
// because absence under DataIncomplete is "unknown", not "confirmed". It pairs
// with the registry's snapshotSourceUnavailableFindings (which covers the
// FALSE_NEGATIVE / silence half). A new rule that reasons on errored data trips
// this gate in CI before it can ship.
//
// Enforces doctor_rule_evaluate_must_consult_snap_errors /
// meta.absence_scope_must_be_explicit at the registry level, across ALL rules,
// not just the ones audited by hand on 2026-06-09.
func TestNoRuleEmitsConfidentFailureOnErroredSnapshot(t *testing.T) {
	r := NewRegistry(Config{})
	snap := fullyErroredSnapshot()

	emitted := map[string]bool{}
	for _, inv := range r.invariants {
		id := inv.ID()
		confident := false
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Errorf("rule %q panicked on a fully-errored snapshot: %v — rules must be nil-safe on empty data", id, rec)
				}
			}()
			for _, f := range inv.Evaluate(snap, r.cfg) {
				if isConfidentFailure(f) {
					confident = true
					break
				}
			}
		}()
		if confident {
			emitted[id] = true
		}
	}

	// (1) Reject new violations: a confident FAIL on errored data that is not
	//     allowlisted means the rule treated an unavailable source as a
	//     confirmed negative.
	var newViolations []string
	for id := range emitted {
		if _, ok := knownSourceIndependentConfidentRules[id]; !ok {
			newViolations = append(newViolations, id)
		}
	}
	sort.Strings(newViolations)
	for _, id := range newViolations {
		t.Errorf("rule %q emits a confident INVARIANT_FAIL on a fully-errored snapshot — it treats an "+
			"unavailable source as a confirmed negative (FALSE_POSITIVE masking bug). Add "+
			"`if snap.HadError(service, rpc) { return nil }` at the top of Evaluate; or, if the rule's "+
			"data source is genuinely not an RPC sub-fetch, add it to "+
			"knownSourceIndependentConfidentRules with a justification.", id)
	}

	// (2) Keep the allowlist honest: an entry that no longer emits must be
	//     removed so the allowlist can only shrink.
	for id, why := range knownSourceIndependentConfidentRules {
		if !emitted[id] {
			t.Errorf("rule %q is allowlisted (%q) but no longer emits a confident FAIL on a fully-errored "+
				"snapshot — remove it from knownSourceIndependentConfidentRules.", id, why)
		}
	}
}
