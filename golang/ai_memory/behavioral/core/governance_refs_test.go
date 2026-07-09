package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// newRefTestService builds a kernel over an in-memory store and empty registry —
// ProposePrinciple's ref validation consults neither the store contents nor the
// registry, so this is sufficient.
func newRefTestService() *Service {
	return New(store.NewMemoryStore(), domain.NewRegistry())
}

// TestProposePrinciple_RejectsMalformedRefs is the Priority-5 + Priority-1 golden
// test: a proposal whose refs look like comma-split prose is rejected at write
// time with a single structured GovernanceError that lists EVERY offender (never
// one-rejection-at-a-time) and never flags the valid ref.
func TestProposePrinciple_RejectsMalformedRefs(t *testing.T) {
	svc := newRefTestService()

	_, err := svc.ProposePrinciple(context.Background(), &api.ProposePrincipleRequest{
		Principle: api.Principle{
			Project: "globular-services",
			Domain:  "cluster_operator",
			Title:   "malformed-refs probe",
			// "incident(foo, bar)" mangled by an upstream comma-split into two refs:
			RequiredEvidence: []api.RequiredEvidenceRef{"incident(foo", "bar)"},
			// a valid authority that must NOT be reported as an offender:
			Authorities: []api.AuthorityRef{"authority.cluster.ai_executor.runtime_state"},
			// prose with a space is also malformed:
			ForbiddenMoves: []api.ForbiddenMoveRef{"forbidden move with space"},
		},
	})

	if err == nil {
		t.Fatal("expected malformed references to be rejected, got nil")
	}

	var ge *api.GovernanceError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *api.GovernanceError, got %T: %v", err, err)
	}
	if ge.Code != api.CodeInvalidReferenceFormat {
		t.Fatalf("expected code %s, got %s", api.CodeInvalidReferenceFormat, ge.Code)
	}

	// Complete-contract: all three malformed refs reported together, in one pass.
	if len(ge.Offenders) != 3 {
		t.Fatalf("expected 3 offenders, got %d: %+v", len(ge.Offenders), ge.Offenders)
	}
	for _, o := range ge.Offenders {
		if o.OffendingValue == "authority.cluster.ai_executor.runtime_state" {
			t.Fatalf("valid ref wrongly flagged as malformed: %+v", o)
		}
	}

	// The rendered message must be self-describing: code, an offending field+value,
	// and the expected format — all in one response.
	msg := ge.Error()
	for _, want := range []string{"INVALID_REFERENCE_FORMAT", "required_evidence", "incident(foo", "expected:"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error message missing %q; full message: %s", want, msg)
		}
	}
}

// TestProposePrinciple_AcceptsCanonicalRefs guards against over-rejection: clean
// canonical catalog ids (dots, underscores, digits) must pass unharmed.
func TestProposePrinciple_AcceptsCanonicalRefs(t *testing.T) {
	svc := newRefTestService()

	resp, err := svc.ProposePrinciple(context.Background(), &api.ProposePrincipleRequest{
		Principle: api.Principle{
			Project:          "globular-services",
			Domain:           "cluster_operator",
			Title:            "canonical-refs probe",
			AppliesWhen:      []api.ConditionRef{"condition.ai_executor.only_subscription_credential_present"},
			Authorities:      []api.AuthorityRef{"authority.cluster.ai_executor.runtime_state"},
			RequiredEvidence: []api.RequiredEvidenceRef{"evidence.ai_executor.autonomous_subscription_drain_risk_20260706"},
			ForbiddenMoves:   []api.ForbiddenMoveRef{"forbidden.allow_cli_in_autonomous_sendprompt"},
		},
	})
	if err != nil {
		t.Fatalf("canonical references must be accepted, got: %v", err)
	}
	if resp == nil || resp.PrincipleID == "" {
		t.Fatalf("expected a created principle id, got %+v", resp)
	}
}
