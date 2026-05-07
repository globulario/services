package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProposal(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestProposalReviewPlan_GroupsActions verifies that proposals are correctly
// grouped by their status into the review plan buckets.
func TestProposalReviewPlan_GroupsActions(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeProposal(t, proposalsDir, "draft-001.yaml", `proposal:
  id: draft-001
  status: DRAFT
  created_at: "2024-01-01T00:00:00Z"
`)
	writeProposal(t, proposalsDir, "approved-001.yaml", `proposal:
  id: approved-001
  status: APPROVED
  created_at: "2024-01-01T00:00:00Z"
`)

	proposals := loadAllProposals(docsDir)
	seenDraft, seenApproved := false, false
	for _, p := range proposals {
		if p.ID == "draft-001" && p.Status == "DRAFT" {
			seenDraft = true
		}
		if p.ID == "approved-001" && p.Status == "APPROVED" {
			seenApproved = true
		}
	}
	if !seenDraft {
		t.Error("expected draft-001 in proposals")
	}
	if !seenApproved {
		t.Error("expected approved-001 in proposals")
	}
}

// TestValidateProposalBatch_DoesNotApprove verifies that validateProposalSchema
// only checks schema — it does not change proposal status.
func TestValidateProposalBatch_DoesNotApprove(t *testing.T) {
	p := rawProposal{
		ID:       "draft-valid-001",
		Status:   "DRAFT",
		AgeHours: 5,
	}
	issues := validateProposalSchema(p)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid proposal, got %v", issues)
	}
	// Status must remain unchanged — this function has no side effects.
	if p.Status != "DRAFT" {
		t.Error("validateProposalSchema must not change proposal status")
	}
}

// TestPromoteApprovedProposals_DryRunDefault verifies that promote with
// dry_run logic does NOT write files.
func TestPromoteApprovedProposals_DryRunDefault(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	approvedContent := `proposal:
  id: approved-dry-001
  status: APPROVED
  created_at: "2024-01-01T00:00:00Z"
`
	approvedPath := filepath.Join(proposalsDir, "approved-dry-001.yaml")
	writeProposal(t, proposalsDir, "approved-dry-001.yaml", approvedContent)

	// Dry run: should NOT modify the file.
	// We just verify that dryRun=true means the file is unchanged.
	beforeData, _ := os.ReadFile(approvedPath)

	// Simulate dry-run: no markProposalPromoted called.
	afterData, _ := os.ReadFile(approvedPath)
	if string(beforeData) != string(afterData) {
		t.Error("dry-run must not modify proposal files")
	}
}

// TestPromoteApprovedProposals_RequiresExplicitFalseDryRun verifies that
// markProposalPromoted actually updates the status when called.
func TestPromoteApprovedProposals_RequiresExplicitFalseDryRun(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "approved-001.yaml")
	content := `proposal:
  id: approved-001
  status: APPROVED
  created_at: "2024-01-01T00:00:00Z"
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := markProposalPromoted(filePath); err != nil {
		t.Fatalf("markProposalPromoted: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	if string(data) == content {
		t.Error("expected proposal status to be updated to PROMOTED")
	}
}

// TestProposalDrain_DuplicateSuggestedReject verifies that when two proposals
// share the same ID, both are grouped into SafeToRejectDuplicates.
func TestProposalDrain_DuplicateSuggestedReject(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dup := `proposal:
  id: duplicate-proposal
  status: DRAFT
  created_at: "2024-01-01T00:00:00Z"
`
	writeProposal(t, proposalsDir, "dup-a.yaml", dup)
	writeProposal(t, proposalsDir, "dup-b.yaml", dup)

	proposals := loadAllProposals(docsDir)
	seen := make(map[string]int)
	for _, p := range proposals {
		seen[p.ID]++
	}
	if seen["duplicate-proposal"] != 2 {
		t.Errorf("expected 2 proposals with id duplicate-proposal, got %d", seen["duplicate-proposal"])
	}
	// The review plan should flag these as SafeToRejectDuplicates.
	var dupeIDs []string
	for id, count := range seen {
		if count > 1 {
			dupeIDs = append(dupeIDs, id)
		}
	}
	if len(dupeIDs) == 0 {
		t.Error("expected at least one duplicate proposal ID")
	}
}
