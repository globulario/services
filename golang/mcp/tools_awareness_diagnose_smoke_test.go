package main

// This file is a smoke test that runs the composer with hand-built sources
// and prints the response so we can inspect what an agent would actually see.
// Run with: go test -run TestDiagnose_SmokeExampleOutput -v ./mcp/

import (
	"encoding/json"
	"testing"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestDiagnose_SmokeExampleOutput(t *testing.T) {
	h := mkHints("apply_package_release reports SUCCESS but installed binary hash drifts from manifest")
	c := &collectedSources{
		briefing: &awarenesspb.BriefingResponse{
			Prose: "Awareness briefing — direct invariants:\n" +
				"- [critical] runtime.success_requires_expected_binary_checksum\n" +
				"- [critical] node_agent.install_claim_requires_binary_proof\n" +
				"Direct failure modes:\n" +
				"- [high] node_agent.apply_package_release_buildid_skip_ignores_checksum",
			ReferencedIds: []string{
				"invariant:runtime.success_requires_expected_binary_checksum",
				"invariant:node_agent.install_claim_requires_binary_proof",
				"failure_mode:node_agent.apply_package_release_buildid_skip_ignores_checksum",
			},
			Status: awarenesspb.BriefingStatus_BRIEFING_STATUS_OK,
		},
		doctorReport: &cluster_doctorpb.ClusterReport{
			Findings: []*cluster_doctorpb.Finding{
				mkFinding(
					"finding-001",
					"runtime.success_requires_expected_binary_checksum",
					"critical",
					"installed binary sha256 differs from manifest entrypoint_checksum on globule-ryzen",
					"service:repository.RepositoryService@globule-ryzen",
				),
				mkFinding(
					"finding-002",
					"some.unrelated.invariant",
					"warn",
					"unrelated warning about disk usage",
					"node:globule-nuc",
				),
				mkFinding(
					"finding-003",
					"",
					"warn",
					"package release apply_package returned SUCCESS without verification",
					"service:repository.RepositoryService",
				),
			},
		},
		driftReport: &cluster_doctorpb.DriftReport{
			TotalDriftCount: 1,
			Items: []*cluster_doctorpb.DriftItem{
				{NodeId: "globule-ryzen", EntityRef: "repository.RepositoryService",
					Category: cluster_doctorpb.DriftCategory_VERSION_MISMATCH,
					Desired: "v1.2.142", Actual: "v1.2.141"},
			},
		},
	}

	resp := buildDiagnoseResponse(h, c)
	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%s", string(out))
}
