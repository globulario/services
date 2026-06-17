package main

import (
	"context"
	"strings"
	"testing"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// TestListRuns_DegradedScyllaReturnsUnavailable locks in the AWG re-audit fix
// (meta.authority_must_express_uncertainty): when ScyllaDB is degraded (nil
// session) ListRuns must surface DEPENDENCY_UNAVAILABLE, not an empty list.
// An empty Total:0 is indistinguishable from "no runs" and was being read by
// readiness consumers as "no active work" exactly when the truth is unknown.
func TestListRuns_DegradedScyllaReturnsUnavailable(t *testing.T) {
	srv := &server{} // nil session -> Scylla degraded
	resp, err := srv.ListRuns(context.Background(), &workflowpb.ListRunsRequest{ClusterId: "c1"})
	if err == nil {
		t.Fatalf("expected DEPENDENCY_UNAVAILABLE error on nil session, got resp=%v", resp)
	}
	if !strings.Contains(err.Error(), "WORKFLOW_DEPENDENCY_UNAVAILABLE") {
		t.Errorf("expected WORKFLOW_DEPENDENCY_UNAVAILABLE, got %v", err)
	}
}

// TestScanDoctorFindings_UnavailableWhenClientNil locks in the AWG re-audit fix
// (meta.absence_scope_must_be_explicit): when the cluster-doctor source cannot
// be observed, its category is marked unavailable so resolveAbsent does NOT age
// open doctor-finding incidents toward RESOLVED. "Not observed because the
// source is down" must not be read as "the finding is gone."
func TestScanDoctorFindings_UnavailableWhenClientNil(t *testing.T) {
	srv := &server{} // doctorClient == nil -> source not wired this scan
	present := map[string]bool{}
	unavailable := map[string]bool{}

	srv.scanDoctorFindings("c1", present, unavailable)

	if !unavailable[incidentCategoryDoctorFinding] {
		t.Error("doctor source unavailable must mark its category unavailable")
	}
	if len(present) != 0 {
		t.Errorf("no findings should be present when source is down, got %d", len(present))
	}
}
