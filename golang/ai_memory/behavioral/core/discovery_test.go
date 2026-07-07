package core

import (
	"context"
	"errors"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// TestListAndResolve_Discovery is the Priority-4 golden test: authorities and
// conditions are discoverable through the API (no grep), and ResolveRef reports
// resolution + kind.
func TestListAndResolve_Discovery(t *testing.T) {
	st := store.NewMemoryStore()
	svc := New(st, domain.NewRegistry())
	ctx := context.Background()

	_ = st.PutAuthority(ctx, &api.Authority{ID: "authority.x.owner", Project: "p", Domain: "d", Title: "Owner", OwnerKind: "service_rpc"})
	_ = st.PutCondition(ctx, &api.Condition{ID: "condition.x.up", Project: "p", Domain: "d", Title: "Up"})
	// A row in a different domain must NOT leak into the scan.
	_ = st.PutAuthority(ctx, &api.Authority{ID: "authority.other", Project: "p", Domain: "other", Title: "Other"})

	la, err := svc.ListAuthorities(ctx, &api.ListAuthoritiesRequest{Project: "p", Domain: "d"})
	if err != nil {
		t.Fatalf("list authorities: %v", err)
	}
	if len(la.Authorities) != 1 || la.Authorities[0].ID != "authority.x.owner" {
		t.Fatalf("expected exactly the in-domain authority, got %+v", la.Authorities)
	}

	lc, err := svc.ListConditions(ctx, &api.ListConditionsRequest{Project: "p", Domain: "d"})
	if err != nil {
		t.Fatalf("list conditions: %v", err)
	}
	if len(lc.Conditions) != 1 || lc.Conditions[0].ID != "condition.x.up" {
		t.Fatalf("expected exactly the in-domain condition, got %+v", lc.Conditions)
	}

	// ResolveRef: authority, condition, and a miss.
	if r, err := svc.ResolveRef(ctx, &api.ResolveRefRequest{Project: "p", Domain: "d", Ref: "authority.x.owner"}); err != nil || !r.Resolved || r.Kind != "authority" {
		t.Fatalf("resolve authority: err=%v resp=%+v", err, r)
	}
	if r, err := svc.ResolveRef(ctx, &api.ResolveRefRequest{Project: "p", Domain: "d", Ref: "condition.x.up"}); err != nil || !r.Resolved || r.Kind != "condition" {
		t.Fatalf("resolve condition: err=%v resp=%+v", err, r)
	}
	if r, err := svc.ResolveRef(ctx, &api.ResolveRefRequest{Project: "p", Domain: "d", Ref: "nope.nothing"}); err != nil || r.Resolved {
		t.Fatalf("resolve miss should be Resolved=false with no error: err=%v resp=%+v", err, r)
	}
}

// TestAmendProposal is the Priority-6 golden test: a PROPOSED principle can be
// amended in place (set-merge refs), the edit resets a prior contradiction check,
// malformed refs are rejected (P5 still applies), and a promoted principle cannot
// be amended.
func TestAmendProposal(t *testing.T) {
	svc := New(store.NewMemoryStore(), domain.NewRegistry())
	ctx := context.Background()

	pr, err := svc.ProposePrinciple(ctx, &api.ProposePrincipleRequest{
		Principle: api.Principle{
			Project: "globular-services", Domain: "cluster_operator", Title: "amend probe",
			ProposedBy: "t", ContradictionChecked: true, // simulate a prior check
			Authorities: []api.AuthorityRef{"authority.a"},
		},
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}

	// Amend: add an authority + a condition; expect the prior contradiction check reset.
	resp, err := svc.AmendProposal(ctx, &api.AmendProposalRequest{
		Project: "globular-services", Domain: "cluster_operator", ID: pr.PrincipleID, Actor: "t",
		AddAuthorityRefs: []string{"authority.b"},
		AddConditionRefs: []string{"condition.c"},
	})
	if err != nil {
		t.Fatalf("amend: %v", err)
	}
	if !resp.ContradictionReset {
		t.Fatal("amending content must invalidate a prior contradiction check")
	}
	if resp.Version != 2 {
		t.Fatalf("expected version bump to 2, got %d", resp.Version)
	}

	// Malformed ref on amend must be rejected (P5 still applies).
	_, err = svc.AmendProposal(ctx, &api.AmendProposalRequest{
		Project: "globular-services", Domain: "cluster_operator", ID: pr.PrincipleID, Actor: "t",
		AddAuthorityRefs: []string{"authority(bad, ref)"},
	})
	var ge *api.GovernanceError
	if !errors.As(err, &ge) || ge.Code != api.CodeInvalidReferenceFormat {
		t.Fatalf("expected INVALID_REFERENCE_FORMAT on malformed amend, got %v", err)
	}

	// Promote is impossible here (missing gate inputs) — but simulate promoted
	// status directly to assert amend refuses a non-PROPOSED principle.
	p, _ := svc.store.GetPrinciple(ctx, "globular-services", "cluster_operator", pr.PrincipleID)
	p.Status = api.StatusPromotedPrinciple
	_ = svc.store.CreatePrinciple(ctx, p)
	_, err = svc.AmendProposal(ctx, &api.AmendProposalRequest{
		Project: "globular-services", Domain: "cluster_operator", ID: pr.PrincipleID, Actor: "t",
		AddAuthorityRefs: []string{"authority.z"},
	})
	if !errors.As(err, &ge) || ge.Code != api.CodeUnsafeOperationRefused {
		t.Fatalf("expected UNSAFE_OPERATION_REFUSED amending a promoted principle, got %v", err)
	}
}
