package cluster_operator

import (
	"strings"
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
		"authority.cluster.release_pipeline.deployed_identity",
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
		"condition.cluster.service.binary_update_intended", "condition.always",
		"condition.cluster.owner_owned_state_write_intended",
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
		"forbidden.cluster.hot_swap_binary_outside_release_pipeline",
		"forbidden.cluster.raw_owner_owned_state_write",
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
		"principle.cluster.deploy_through_release_pipeline_never_hot_swap",
		"principle.cluster.no_raw_owner_owned_state_write",
	}
	for _, id := range wantPrinc {
		if !contains(p.PrincipleIDs(), id) {
			t.Errorf("missing hand-authored principle %q", id)
		}
	}
}

// The raw-owner-owned-state-write rule (Governed Operation Gateway, Slice 1) must be
// reachable by the runtime gate: the forbidden move must carry the concrete footgun
// action_aliases (the gate matches an agent's natural action name against these — see
// forbiddenRefMatches), and a sentinel principle must bind it so it is always evaluated.
func TestRawOwnerStateWriteRuleIsDiscoverable(t *testing.T) {
	p := MustNew()

	const fmID = "forbidden.cluster.raw_owner_owned_state_write"
	var aliases string
	for _, fm := range p.ForbiddenMoves() {
		if fm.ID == fmID {
			aliases = fm.Fields["action_aliases"]
		}
	}
	if aliases == "" {
		t.Fatalf("forbidden move %q missing or has no action_aliases — the gate would have no reach", fmID)
	}
	// The specific footgun action names an agent/CLI/script would present must be
	// covered, or a raw write slips past the gate under a natural name.
	wantAlias := []string{
		"etcdctl_put", "etcd_delete", "mcp_raw_write", "write_desired_state_directly",
		"patch_resolved_version", "patch_cache_digest", "set_infra_version_raw",
		"services_desired_set_force_cross_kind", "nodeagent_installed_set_raw",
	}
	for _, a := range wantAlias {
		if !strings.Contains(aliases, a) {
			t.Errorf("action_aliases for %q missing %q (gate would not match it); have %q", fmID, a, aliases)
		}
	}

	// A sentinel principle (condition.always) must bind the forbidden move so it is
	// evaluated on every check, and it must impose no situational evidence gate — it
	// blocks purely on forbidden-move match (sentinel applicability != situational gate).
	const pID = "principle.cluster.no_raw_owner_owned_state_write"
	var bound bool
	for _, ps := range p.PrincipleSeeds() {
		if ps.ID != pID {
			continue
		}
		bound = true
		if !contains(ps.AppliesWhen, "condition.always") {
			t.Errorf("%q must apply under condition.always (sentinel) so it is always evaluated", pID)
		}
		if !contains(ps.ForbiddenMoves, fmID) {
			t.Errorf("%q must bind forbidden move %q", pID, fmID)
		}
		if len(ps.RequiredEvidence) != 0 {
			t.Errorf("%q must impose no situational evidence gate (blocks on forbidden-move match), got %v", pID, ps.RequiredEvidence)
		}
	}
	if !bound {
		t.Fatalf("sentinel principle %q not found", pID)
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
