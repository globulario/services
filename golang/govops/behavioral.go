package govops

import (
	"sort"
	"strings"

	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

// rawOwnerWriteForbiddenID is the behavioral forbidden-move that names a raw
// owner-owned-state write — the move the Governed Operation Gateway exists to
// refuse (the cross-kind / raw-etcd desired-write class behind the a399ebea scar).
// It is authored in the cluster_operator behavioral seed and bound by
// principle.cluster.no_raw_owner_owned_state_write.
const rawOwnerWriteForbiddenID = "forbidden.cluster.raw_owner_owned_state_write"

// rawOwnerWriteAliases is the deterministic, in-process set of action verbs that
// name a raw owner-owned-state write.
//
// CONTRACT — govops_behavioral_refusal_uses_compiled_seed_not_live_rpc:
// it is loaded ONCE, at package init, from the COMPILED cluster_operator behavioral
// seed pack (cluster_operator.MustNew()) — the same authored artifact
// behavioral-memory promotes. It is NOT fetched from a live ai-memory RPC and NOT
// read from the behavioral store/DB. Refusal is therefore deterministic and local:
// it never depends on behavioral-memory availability, so it can neither fail open
// when ai-memory is down nor block deterministic convergence when it is. Memory
// supplies the rule; govops enforces it. Do NOT replace this compiled match with a
// live CheckAction call — a live RPC may only ever ENRICH this gate, never be the
// authority required to refuse a known forbidden move.
var rawOwnerWriteAliases = loadForbiddenAliases(rawOwnerWriteForbiddenID)

// loadForbiddenAliases reads the action_aliases of one forbidden move from the
// compiled cluster_operator seed pack into a normalized lookup set.
func loadForbiddenAliases(forbiddenID string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, fm := range cluster_operator.MustNew().ForbiddenMoves() {
		if fm.ID != forbiddenID {
			continue
		}
		for _, a := range strings.Split(fm.Fields["action_aliases"], ",") {
			if n := normalizeAction(a); n != "" {
				set[n] = struct{}{}
			}
		}
	}
	return set
}

func normalizeAction(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// matchRawOwnerWrite returns the forbidden-move id when the action names a raw
// owner-owned-state write (by any of its authored aliases), or "" otherwise. The
// match is exact on a normalized verb, so it refuses the known forbidden move
// without over-blocking unrelated, owner-routed actions.
func matchRawOwnerWrite(action string) string {
	if _, ok := rawOwnerWriteAliases[normalizeAction(action)]; ok {
		return rawOwnerWriteForbiddenID
	}
	return ""
}

// rawOwnerWriteAliasList returns the loaded aliases, sorted. Exposed for the
// boundary guard that proves govops reads the real compiled seed (not a live RPC,
// not a hand-copied shadow).
func rawOwnerWriteAliasList() []string {
	out := make([]string, 0, len(rawOwnerWriteAliases))
	for a := range rawOwnerWriteAliases {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// LedgerEntryFor projects a validated request + decision onto an
// OperationLedgerEntry, recording the behavioral rule ids that grounded a refusal
// in behavioral_rules and the terminal result. A behavioral refusal thus becomes a
// durable, queryable fact (LedgerFilter.OnlyRefused), not an ephemeral verdict.
func LedgerEntryFor(r *pb.OperationRequest, d Decision) *pb.OperationLedgerEntry {
	e := &pb.OperationLedgerEntry{
		OperationId:    r.GetId(),
		Actor:          r.GetActor().String(),
		Command:        r.GetAction(),
		TargetOwner:    r.GetTarget().GetOwner(),
		TargetResource: r.GetTarget().GetResourceId(),
		Result:         resultForDecision(d),
	}
	seen := map[string]struct{}{}
	for _, v := range d.Violations {
		if v.Rule == "" {
			continue
		}
		if _, dup := seen[v.Rule]; dup {
			continue
		}
		seen[v.Rule] = struct{}{}
		e.BehavioralRules = append(e.BehavioralRules, v.Rule)
	}
	return e
}

func resultForDecision(d Decision) pb.OperationResult {
	if d.Refused() {
		return pb.OperationResult_REFUSED
	}
	return pb.OperationResult_ALLOWED
}

// Execute runs the approved owner-path callback ONLY when the decision permits the
// mutation. A refused decision — including a behavioral forbidden-move — returns
// REFUSED and the owner path is NEVER invoked. This is the enforcement seam:
// validation is not advisory; a refusal stops the operation before it can touch
// owner-owned state.
func Execute(d Decision, ownerPath func() error) (pb.OperationResult, error) {
	if d.Refused() {
		return pb.OperationResult_REFUSED, nil
	}
	if err := ownerPath(); err != nil {
		return pb.OperationResult_FAILED, err
	}
	return pb.OperationResult_COMPLETED, nil
}
