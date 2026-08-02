package cluster_operator

// Commit 4D: the two requirement IDs the satisfaction catalog keys on must be
// DECLARED in the pack, not merely referenced by a rule.
//
// Until now the catalog named requirement IDs the domain pack had never heard
// of. Nothing failed, because the catalog and the pack are validated
// separately — but Pack.validate() rejects a principle referencing an unknown
// required-evidence id, so no principle could have declared either requirement.
// The governance path was unreachable from the principle side while looking
// complete from the evidence side.

import (
	"sort"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
)

// The exact ids the satisfaction catalog rules point at. Written out literally
// rather than read from the catalog: if a rule's RequirementID is changed by
// accident, this test must fail rather than agree with the change.
const (
	reqDoctorFindingObserved = "evidence.doctor.finding_observed"
	reqRemediationVerified   = "evidence.remediation.fresh_convergence_verification"
)

func TestRequiredEvidence_CatalogRequirementsAreDeclared(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack must load and validate: %v", err)
	}

	declared := map[string]bool{}
	for _, id := range p.RequiredEvidenceIDs() {
		declared[id] = true
	}
	for _, id := range []string{reqDoctorFindingObserved, reqRemediationVerified} {
		if !declared[id] {
			t.Errorf("requirement %q is used by the satisfaction catalog but not declared in the pack — "+
				"no principle could reference it", id)
		}
	}
}

// Every requirement the catalog can satisfy must be declared, and every rule
// must point at a declared requirement. This catches the drift directly rather
// than relying on the two ids above staying current.
func TestRequiredEvidence_EverySatisfactionRuleTargetsADeclaredRequirement(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	declared := map[string]bool{}
	for _, id := range p.RequiredEvidenceIDs() {
		declared[id] = true
	}
	for _, rule := range satisfactionCatalog {
		if !declared[rule.RequirementID] {
			t.Errorf("rule %q satisfies %q, which the pack does not declare", rule.ID, rule.RequirementID)
		}
	}
}

// A principle may now declare either requirement — the thing that was
// impossible before.
func TestRequiredEvidence_PrincipleMayDeclareEitherRequirement(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	for _, req := range []string{reqDoctorFindingObserved, reqRemediationVerified} {
		t.Run(req, func(t *testing.T) {
			withPrinciple := &Pack{catalogs: p.catalogs}
			withPrinciple.catalogs.Principles = append(
				append([]domain.PrincipleSeed{}, p.catalogs.Principles...),
				domain.PrincipleSeed{
					ID:               "principle.test.requires_" + req,
					Title:            "test principle",
					RequiredEvidence: []string{req},
				},
			)
			if err := withPrinciple.validate(); err != nil {
				t.Fatalf("a principle declaring %q must validate: %v", req, err)
			}
		})
	}
}

// Misspelled ids must still fail — the point of declaring them is that the
// reference is checked, not that everything now passes.
func TestRequiredEvidence_MisspelledReferenceFails(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	bad := &Pack{catalogs: p.catalogs}
	bad.catalogs.Principles = append(
		append([]domain.PrincipleSeed{}, p.catalogs.Principles...),
		domain.PrincipleSeed{
			ID:    "principle.test.typo",
			Title: "typo",
			// One character off.
			RequiredEvidence: []string{"evidence.remediation.fresh_convergence_verifcation"},
		},
	)
	if err := bad.validate(); err == nil {
		t.Fatal("a misspelled required-evidence reference must fail validation")
	}
}

func TestRequiredEvidence_DuplicateIDFails(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	dup := &Pack{catalogs: p.catalogs}
	dup.catalogs.RequiredEvidence = append(
		append([]domain.CatalogEntry{}, p.catalogs.RequiredEvidence...),
		domain.CatalogEntry{ID: reqRemediationVerified, Title: "duplicate"},
	)
	if err := dup.validate(); err == nil {
		t.Fatal("a duplicate required-evidence id must fail validation")
	}
}

// The push-sourced requirements carry no probe_ref on purpose: no prober exists
// for them, and a plausible-looking ref would advertise a capability the system
// does not have. Asserted so a later author does not "fix" the blank.
func TestRequiredEvidence_PushSourcedRequirementsDeclareNoProbe(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	probes := p.EvidenceProbes()
	for _, id := range []string{reqDoctorFindingObserved, reqRemediationVerified} {
		spec, ok := probes[id]
		if !ok {
			t.Fatalf("%q must appear in the probe spec map", id)
		}
		if spec.ProbeRef != "" {
			t.Errorf("%q declares probe_ref %q, but nothing can execute it — "+
				"this evidence is pushed by a producer, not pulled by a probe", id, spec.ProbeRef)
		}
		if spec.Lane != "runtime_required" {
			t.Errorf("%q lane = %q, want runtime_required", id, spec.Lane)
		}
	}
}

// No unrelated requirements were added.
//
// Scoped to the hand-authored seed rather than the merged catalog: the pack also
// loads the compiler-generated corpus, whose contents change with the ops
// knowledge base. Asserting on the merge would make this test fail for reasons
// that have nothing to do with what a human declared.
func TestRequiredEvidence_OnlyTheTwoRequirementsWereAdded(t *testing.T) {
	seed, err := parseEntries(seedFS, "seed/required_evidence.yaml")
	if err != nil {
		t.Fatalf("parse seed: %v", err)
	}
	got := make([]string, len(seed))
	for i, e := range seed {
		got[i] = e.ID
	}
	sort.Strings(got)

	want := []string{
		"evidence.cluster.envoy.route_config",
		"evidence.cluster.etcd.alarm_status",
		"evidence.cluster.etcd.compaction_state",
		"evidence.cluster.etcd.member_health",
		"evidence.cluster.human.irreversible_approval",
		"evidence.cluster.minio.pool_health",
		"evidence.cluster.owner_service.desired_state",
		"evidence.cluster.owner_service.observed_state",
		"evidence.cluster.scylla.schema_agreement",
		reqDoctorFindingObserved,
		reqRemediationVerified,
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("seed required-evidence catalog has %d entries, want %d:\n got=%v\nwant=%v",
			len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}
