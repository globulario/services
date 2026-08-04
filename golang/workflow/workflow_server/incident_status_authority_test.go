package main

import (
	"testing"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// F5: incident classification must use the AUTHORITATIVE RunStatus enum.
//
// resp.Status is produced by legacyRunStatusString, which collapses the entire
// enum onto "SUCCEEDED" or "FAILED" to satisfy ExecuteWorkflowResponse's
// documented contract. projectIncident used to branch on that string, so every
// DEFERRED and BLOCKED run arrived labelled "FAILED" and was recorded as a
// terminal failure in the AI learning dataset — a run waiting on a prerequisite
// became evidence of breakage.
//
// The old guard read `resp.Status != "FAILED" && resp.Status != "BLOCKED"`,
// which shows the intent was already to treat BLOCKED separately. But
// legacyRunStatusString can never emit "BLOCKED", so that branch was dead code.

func TestIncidentEligibility_OnlyTerminalFailure(t *testing.T) {
	for _, tc := range []struct {
		status workflowpb.RunStatus
		want   bool
		why    string
	}{
		{workflowpb.RunStatus_RUN_STATUS_FAILED, true, "terminal failure is the only incident-eligible status"},
		{workflowpb.RunStatus_RUN_STATUS_DEFERRED, false, "deferred is waiting, not broken"},
		{workflowpb.RunStatus_RUN_STATUS_BLOCKED, false, "blocked is waiting, not broken"},
		{workflowpb.RunStatus_RUN_STATUS_SUCCEEDED, false, "success is not a failure"},
		{workflowpb.RunStatus_RUN_STATUS_EXECUTING, false, "non-terminal must not project"},
		{workflowpb.RunStatus_RUN_STATUS_PENDING, false, "non-terminal must not project"},
		{workflowpb.RunStatus_RUN_STATUS_RETRYING, false, "retrying is not a terminal failure"},
		{workflowpb.RunStatus_RUN_STATUS_CANCELED, false, "cancellation is not a failure incident"},
		{workflowpb.RunStatus_RUN_STATUS_SUPERSEDED, false, "superseded is not a failure incident"},
		{workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK, false, "rollback is not a terminal failure"},
		{workflowpb.RunStatus_RUN_STATUS_UNKNOWN, false, "unclassified must not default to failure"},
	} {
		if got := incidentEligibleStatus(tc.status); got != tc.want {
			t.Errorf("incidentEligibleStatus(%s) = %v, want %v — %s", tc.status, got, tc.want, tc.why)
		}
	}
}

// The legacy mapper stays lossy BY DESIGN for the external contract — that is
// precisely why internal classification must not consume it.
func TestLegacyRenderingUnchanged(t *testing.T) {
	if got := legacyRunStatusString(workflowpb.RunStatus_RUN_STATUS_SUCCEEDED); got != "SUCCEEDED" {
		t.Errorf("external contract changed: SUCCEEDED rendered as %q", got)
	}
	for _, s := range []workflowpb.RunStatus{
		workflowpb.RunStatus_RUN_STATUS_FAILED,
		workflowpb.RunStatus_RUN_STATUS_DEFERRED,
		workflowpb.RunStatus_RUN_STATUS_BLOCKED,
	} {
		if got := legacyRunStatusString(s); got != "FAILED" {
			t.Errorf("external contract changed: %s rendered as %q, want FAILED", s, got)
		}
	}
	// The very collapse that made the old classification wrong:
	if legacyRunStatusString(workflowpb.RunStatus_RUN_STATUS_DEFERRED) !=
		legacyRunStatusString(workflowpb.RunStatus_RUN_STATUS_FAILED) {
		t.Fatal("sanity: DEFERRED and FAILED must be indistinguishable in the legacy string")
	}
	// ...yet they must be distinguishable internally.
	if incidentEligibleStatus(workflowpb.RunStatus_RUN_STATUS_DEFERRED) ==
		incidentEligibleStatus(workflowpb.RunStatus_RUN_STATUS_FAILED) {
		t.Error("internal classification must distinguish DEFERRED from FAILED")
	}
}

// Changing the legacy mapper must not be able to alter internal classification.
func TestLegacyMapperCannotAffectIncidentClassification(t *testing.T) {
	// incidentEligibleStatus takes the enum only; there is no path from the
	// legacy string into it. Assert the label helper also bypasses the mapper.
	if got := incidentStatusLabel(workflowpb.RunStatus_RUN_STATUS_DEFERRED); got != "DEFERRED" {
		t.Errorf("incident label must render the real status; got %q", got)
	}
	if got := incidentStatusLabel(workflowpb.RunStatus_RUN_STATUS_BLOCKED); got != "BLOCKED" {
		t.Errorf("incident label must render the real status; got %q", got)
	}
}

// Structural ratchet: projectIncident must not read resp.Status for
// classification.
func TestProjectIncident_DoesNotClassifyFromLegacyString(t *testing.T) {
	src := stripLineComments(readWorkflowSource(t, "incident_projection.go"))
	for _, bad := range []string{
		`resp.Status != "FAILED"`,
		`resp.Status == "FAILED"`,
		`resp.Status != "BLOCKED"`,
	} {
		if containsStr(src, bad) {
			t.Errorf("projectIncident must classify from the RunStatus enum, not the legacy "+
				"response string; found %q", bad)
		}
	}
	if !containsStr(src, "incidentEligibleStatus(status)") {
		t.Error("projectIncident must gate on incidentEligibleStatus(status)")
	}
}
