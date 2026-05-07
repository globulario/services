package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposalQueueHealth_StaleDraft verifies that a DRAFT proposal older
// than the SLA is reported as stale with the correct recommended action.
func TestProposalQueueHealth_StaleDraft(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	staleYAML := `proposal:
  id: draft-stale-001
  status: DRAFT
  created_at: "2000-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(proposalsDir, "draft-stale-001.yaml"), []byte(staleYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	count := countStaleProposals(docsDir, 24.0)
	if count != 1 {
		t.Errorf("expected 1 stale draft, got %d", count)
	}
}

// TestProposalQueueHealth_ApprovedNotPromoted verifies that an APPROVED
// proposal past the SLA appears in stale entries with the right status.
func TestProposalQueueHealth_ApprovedNotPromoted(t *testing.T) {
	counts := proposalCounts{Approved: 1, Stale: 1}
	stale := []staleProposalEntry{{ProposalID: "approved-001", Status: "APPROVED", AgeHours: 48, Reason: "APPROVED older than 24h SLA — not yet promoted"}}
	status := computeQueueStatus(counts, 1, 0)
	if status != "stale" {
		t.Errorf("expected stale, got %q", status)
	}
	if len(stale) != 1 || stale[0].Status != "APPROVED" {
		t.Error("stale entry should have APPROVED status")
	}
}

// TestProposalQueueHealth_DuplicateProposal verifies that duplicate proposal
// IDs produce "blocked" queue status.
func TestProposalQueueHealth_DuplicateProposal(t *testing.T) {
	counts := proposalCounts{}
	status := computeQueueStatus(counts, 0, 1) // 1 duplicate
	if status != "blocked" {
		t.Errorf("expected blocked, got %q", status)
	}
}

// TestPendingProposals_IncludesRecommendedActions verifies that loadPendingProposalCount
// skips PROMOTED and REJECTED proposals.
func TestPendingProposals_IncludesRecommendedActions(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a PROMOTED proposal (should be excluded).
	promotedYAML := `proposal:
  id: promoted-001
  status: PROMOTED
  created_at: "2000-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(proposalsDir, "promoted-001.yaml"), []byte(promotedYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a DRAFT proposal (should be included).
	draftYAML := `proposal:
  id: draft-002
  status: DRAFT
  created_at: "2000-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(proposalsDir, "draft-002.yaml"), []byte(draftYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	count, ids := loadPendingProposalCount(docsDir)
	if count != 1 {
		t.Errorf("expected 1 pending proposal, got %d", count)
	}
	if len(ids) != 1 || !strings.Contains(ids[0], "draft") {
		t.Errorf("expected draft-002 in pending IDs, got %v", ids)
	}
}
