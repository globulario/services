package core

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// TestPromotePrinciple_BlockedReturnsSatisfactionRecipe is the Priority-3 golden
// test: a blocked promotion must return the COMPLETE satisfaction recipe — a
// summary plus a step per unsatisfied requirement, each carrying actionable
// next_operations — so an agent never discovers the contract one rejection at a
// time. The gate must still refuse (enforcement unchanged).
func TestPromotePrinciple_BlockedReturnsSatisfactionRecipe(t *testing.T) {
	svc := New(store.NewMemoryStore(), domain.NewRegistry())
	ctx := context.Background()

	// A bare proposal: valid refs (so it is accepted) but nothing the gate needs
	// (no evidence, no authority, no conditions, no revocation rule, etc.).
	pr, err := svc.ProposePrinciple(ctx, &api.ProposePrincipleRequest{
		Principle: api.Principle{
			Project:    "globular-services",
			Domain:     "cluster_operator",
			Title:      "recipe probe",
			ProposedBy: "test-agent",
		},
	})
	if err != nil {
		t.Fatalf("propose failed: %v", err)
	}

	resp, err := svc.PromotePrinciple(ctx, &api.PromotePrincipleRequest{
		Project:     "globular-services",
		Domain:      "cluster_operator",
		PrincipleID: pr.PrincipleID,
		Actor:       "test-agent",
	})
	if err != nil {
		t.Fatalf("promote returned transport error (should be a blocked decision, not an error): %v", err)
	}

	// Enforcement unchanged: the gate must still refuse.
	if resp.Decision != api.PromotionBlocked {
		t.Fatalf("expected PromotionBlocked, got %v", resp.Decision)
	}

	rec := resp.Record
	if len(rec.SatisfactionSteps) == 0 {
		t.Fatal("blocked decision must carry satisfaction_steps, got none")
	}
	if rec.SatisfactionSummary == "" {
		t.Fatal("blocked decision must carry a satisfaction_summary")
	}

	// Every step must be actionable: a requirement key and a how-to.
	haveAuthority := false
	for _, s := range rec.SatisfactionSteps {
		if s.Requirement == "" || s.HowToSatisfy == "" {
			t.Fatalf("step missing requirement/how_to_satisfy: %+v", s)
		}
		if s.Requirement == "mapped_authority" {
			haveAuthority = true
			if len(s.NextOperations) == 0 {
				t.Fatalf("mapped_authority step must carry next_operations: %+v", s)
			}
			joined := strings.Join(s.NextOperations, " ")
			// The recipe must point at the real discovery + amend tools.
			if !strings.Contains(joined, "behavioral_list_authorities") || !strings.Contains(joined, "behavioral_amend_proposal") {
				t.Fatalf("mapped_authority step should point at discovery + amend tools, got: %s", joined)
			}
		}
	}
	if !haveAuthority {
		t.Fatalf("expected a mapped_authority step among: %+v", rec.SatisfactionSteps)
	}

	// The summary must enumerate the requirement count.
	if !strings.Contains(rec.SatisfactionSummary, "requirement(s) to satisfy") {
		t.Fatalf("unexpected summary: %q", rec.SatisfactionSummary)
	}
}
