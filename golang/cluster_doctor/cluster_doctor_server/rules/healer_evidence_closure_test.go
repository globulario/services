package rules

import (
	"context"
	"strings"
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type evidenceClosureDispatcher struct {
	calls   int
	dryRuns []bool
}

func (d *evidenceClosureDispatcher) Dispatch(_ context.Context, _ Finding, _ string, dryRun bool) (bool, string, error) {
	d.calls++
	d.dryRuns = append(d.dryRuns, dryRun)
	return true, "rem-evidence-closure", nil
}

func reducedHarvestEvidence() *cluster_doctorpb.Evidence {
	return &cluster_doctorpb.Evidence{
		SourceService: "cluster_doctor",
		SourceRpc:     "reduced_harvest",
		KeyValues: map[string]string{
			"missing_sources": "workflow.ListWorkflowRuns",
		},
	}
}

func autoPolicy(invariantID string) HealRule {
	return HealRule{
		InvariantID: invariantID,
		Disposition: HealAuto,
		AutoAction:  "synthetic_action",
	}
}

func TestHealerReducedHarvest_UnrelatedFailureKeepsConclusiveFindingEligible(t *testing.T) {
	dispatcher := &evidenceClosureDispatcher{}
	h := &Healer{
		Dispatcher:   dispatcher,
		PolicyLookup: autoPolicy,
	}

	report := h.Evaluate(context.Background(), []Finding{{
		FindingID:       "finding-conclusive",
		InvariantID:     "node.systemd.units_running",
		EntityRef:       "node-1/globular-torrent.service",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			{
				SourceService: "node_agent",
				SourceRpc:     "GetInventory",
				KeyValues: map[string]string{
					"node_id":   "node-1",
					"unit_name": "globular-torrent.service",
				},
			},
			reducedHarvestEvidence(),
		},
	}})

	if dispatcher.calls != 1 {
		t.Fatalf("dispatcher calls = %d, want 1: an unrelated workflow failure must not veto target inventory evidence", dispatcher.calls)
	}
	if report.AutoFixed != 1 {
		t.Fatalf("AutoFixed = %d, want 1", report.AutoFixed)
	}
}

func TestHealerReducedHarvest_CompromisedFindingIsRefusedBeforeDispatch(t *testing.T) {
	dispatcher := &evidenceClosureDispatcher{}
	h := &Healer{
		Dispatcher:   dispatcher,
		PolicyLookup: autoPolicy,
	}

	report := h.Evaluate(context.Background(), []Finding{{
		FindingID:       "finding-unknown",
		InvariantID:     "node.systemd.units_running",
		EntityRef:       "node-1/globular-torrent.service",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		CheckError:      "verdict downgraded to UNKNOWN: evidence source unavailable this sweep: node_agent.GetInventory",
		Evidence:        []*cluster_doctorpb.Evidence{reducedHarvestEvidence()},
	}})

	if dispatcher.calls != 0 {
		t.Fatalf("dispatcher calls = %d, want 0: compromised target evidence must fail closed", dispatcher.calls)
	}
	if len(report.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(report.Results))
	}
	if !strings.Contains(report.Results[0].Error, "evidence closure refused") {
		t.Fatalf("error = %q, want explicit evidence-closure refusal", report.Results[0].Error)
	}
}

func TestHealerReducedHarvest_CompromisedFindingDryRunReachesCentralGate(t *testing.T) {
	dispatcher := &evidenceClosureDispatcher{}
	h := &Healer{
		DryRun:      true,
		Dispatcher:  dispatcher,
		PolicyLookup: autoPolicy,
	}

	h.Evaluate(context.Background(), []Finding{{
		FindingID:       "finding-unknown-dry-run",
		InvariantID:     "node.systemd.units_running",
		EntityRef:       "node-1/globular-torrent.service",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		CheckError:      "target inventory unavailable",
		Evidence:        []*cluster_doctorpb.Evidence{reducedHarvestEvidence()},
	}})

	if dispatcher.calls != 1 {
		t.Fatalf("dispatcher calls = %d, want 1: dry-run must traverse the central gate for rehearsal and audit", dispatcher.calls)
	}
	if len(dispatcher.dryRuns) != 1 || !dispatcher.dryRuns[0] {
		t.Fatalf("dispatcher dry-run flags = %v, want [true]", dispatcher.dryRuns)
	}
}
