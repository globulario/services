package cluster_operator

import (
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
)

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// New() parses + validates the embedded seed (refs resolve, ids unique,
// generative pairing present) — a passing New is itself a validation test.
func TestPackNewValidates(t *testing.T) {
	if _, err := New(); err != nil {
		t.Fatalf("New: %v", err)
	}
}

// The pack registers with the generic domain registry.
func TestPackRegistersWithRegistry(t *testing.T) {
	reg := domain.NewRegistry()
	reg.Register(MustNew())
	d, ok := reg.Lookup(DomainName)
	if !ok {
		t.Fatalf("pack %q not found in registry", DomainName)
	}
	if d.Name() != DomainName {
		t.Errorf("name = %q, want %q", d.Name(), DomainName)
	}
}

// All §3 catalog ids are present.
func TestExpectedCatalogIDsPresent(t *testing.T) {
	p := MustNew()
	wantAuth := []string{
		"authority.cluster.etcd.member_health", "authority.cluster.scylla.schema_agreement",
		"authority.cluster.minio.pool_health", "authority.cluster.envoy.route_config",
		"authority.cluster.owner_service.runtime_state", "authority.cluster.human.irreversible_ops",
	}
	for _, id := range wantAuth {
		if !contains(p.AuthorityIDs(), id) {
			t.Errorf("missing authority %q", id)
		}
	}
	wantCond := []string{
		"condition.cluster.etcd.nospace_alarm", "condition.cluster.scylla.schema_disagreement",
		"condition.cluster.minio.pool_degraded", "condition.cluster.envoy.route_missing",
		"condition.cluster.service.desired_observed_mismatch", "condition.cluster.node_removal_requested",
	}
	for _, id := range wantCond {
		if !contains(p.ConditionIDs(), id) {
			t.Errorf("missing condition %q", id)
		}
	}
	wantForbidden := []string{
		"forbidden.cluster.restart_before_quorum_check",
		"forbidden.cluster.claim_recovery_without_authoritative_evidence",
		"forbidden.cluster.direct_runtime_mutation_without_owner_authority",
		"forbidden.cluster.minio_topology_change_without_pool_authority",
		"forbidden.cluster.controller_executes_local_mutation",
	}
	for _, id := range wantForbidden {
		if !contains(p.ForbiddenMoveIDs(), id) {
			t.Errorf("missing forbidden move %q", id)
		}
	}
	// The 9 hand-authored required-evidence refs must all be present (the merged
	// total also includes generated entries).
	wantReq := []string{
		"evidence.cluster.etcd.alarm_status", "evidence.cluster.etcd.member_health",
		"evidence.cluster.etcd.compaction_state", "evidence.cluster.owner_service.desired_state",
		"evidence.cluster.owner_service.observed_state",
	}
	for _, id := range wantReq {
		if !contains(p.RequiredEvidenceIDs(), id) {
			t.Errorf("missing hand-authored required evidence %q", id)
		}
	}
	// The 4 hand-authored principles must all be present.
	wantPrinc := []string{
		"principle.cluster.no_recovery_claim_without_authoritative_evidence",
		"principle.cluster.preserve_quorum_before_restart_under_etcd_pressure",
		"principle.cluster.preserve_owner_executor_boundary",
		"principle.cluster.treat_minio_topology_as_stateful_authority",
	}
	for _, id := range wantPrinc {
		if !contains(p.PrincipleIDs(), id) {
			t.Errorf("missing hand-authored principle %q", id)
		}
	}
}

// Every forbidden move has a paired generative behavior (mandatory).
func TestEveryForbiddenMoveHasGenerativeBehavior(t *testing.T) {
	p := MustNew()
	for _, id := range p.ForbiddenMoveIDs() {
		if p.GenerativeBehavior(id) == "" {
			t.Errorf("forbidden move %q has no paired generative behavior", id)
		}
	}
}

// Seed principles carry lineage (source_refs / generated_from) and a generative
// recommended_action.
func TestSeedPrinciplesHaveLineageAndGenerativeBehavior(t *testing.T) {
	p := MustNew()
	for _, ps := range p.PrincipleSeeds() {
		if len(ps.SourceRefs) == 0 {
			t.Errorf("principle %q missing source_refs", ps.ID)
		}
		if len(ps.GeneratedFrom) == 0 {
			t.Errorf("principle %q missing generated_from", ps.ID)
		}
		if ps.RecommendedAction == "" {
			t.Errorf("principle %q missing generative recommended_action", ps.ID)
		}
		if ps.RiskLevel == "" || ps.RevocationRule == "" || ps.PromotionReason == "" {
			t.Errorf("principle %q missing a gate field (risk/revocation/promotion_reason)", ps.ID)
		}
	}
}

// A malformed seed (dangling ref) is rejected by validate().
func TestValidateRejectsDanglingRef(t *testing.T) {
	p := &Pack{catalogs: domain.Catalogs{
		Conditions: []domain.CatalogEntry{{ID: "c1"}},
		Principles: []domain.PrincipleSeed{{ID: "p1", AppliesWhen: []string{"c-missing"}}},
	}}
	if err := p.validate(); err == nil {
		t.Fatal("validate accepted a principle with a dangling condition ref")
	}
}
