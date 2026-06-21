package main

import (
	"testing"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

// fpReq builds a ProcessIncidentRequest with the given rule, event, and a
// service embedded in the trigger event data.
func fpReq(incidentID, ruleID, event, service string) *ai_executorpb.ProcessIncidentRequest {
	return &ai_executorpb.ProcessIncidentRequest{
		IncidentId:       incidentID,
		RuleId:           ruleID,
		TriggerEventName: event,
		TriggerEventData: []byte(`{"service":"` + service + `"}`),
	}
}

// TestIncidentFingerprint_StableAcrossOccurrences is the core dedup invariant:
// two occurrences of the same problem (same rule + event + service) but with
// DIFFERENT incident_ids must produce the SAME fingerprint. If this regresses,
// every repeat looks new and the ai_executor re-runs a full LLM diagnosis on
// each one — the workflow-storm token drain that motivated the ledger.
func TestIncidentFingerprint_StableAcrossOccurrences(t *testing.T) {
	a := fpReq("incident-aaaa", "workflow-run-failed", "workflow.run.failed", "workflow")
	b := fpReq("incident-bbbb", "workflow-run-failed", "workflow.run.failed", "workflow")

	if incidentFingerprint(a) != incidentFingerprint(b) {
		t.Fatalf("same signature, different incident_id must share a fingerprint: %s != %s",
			incidentFingerprint(a), incidentFingerprint(b))
	}
}

// TestIncidentFingerprint_DistinguishesSignatures ensures the fingerprint is not
// so coarse that genuinely different problems collapse together (which would
// suppress real diagnoses). Rule, event, and service each change the signature.
func TestIncidentFingerprint_DistinguishesSignatures(t *testing.T) {
	base := fpReq("i1", "workflow-run-failed", "workflow.run.failed", "workflow")
	cases := map[string]*ai_executorpb.ProcessIncidentRequest{
		"different rule":    fpReq("i2", "service-crash", "workflow.run.failed", "workflow"),
		"different event":   fpReq("i3", "workflow-run-failed", "service.exited", "workflow"),
		"different service": fpReq("i4", "workflow-run-failed", "workflow.run.failed", "repository"),
	}
	baseFP := incidentFingerprint(base)
	for name, req := range cases {
		if incidentFingerprint(req) == baseFP {
			t.Errorf("%s should change the fingerprint but did not", name)
		}
	}
}

// TestIncidentFingerprint_UnitFallback verifies the signature falls back to the
// "unit" field when "service" is absent, so systemd-unit incidents still dedup.
func TestIncidentFingerprint_UnitFallback(t *testing.T) {
	withUnit := &ai_executorpb.ProcessIncidentRequest{
		IncidentId:       "i1",
		RuleId:           "service-crash",
		TriggerEventName: "service.exited",
		TriggerEventData: []byte(`{"unit":"globular-repository.service"}`),
	}
	other := &ai_executorpb.ProcessIncidentRequest{
		IncidentId:       "i2",
		RuleId:           "service-crash",
		TriggerEventName: "service.exited",
		TriggerEventData: []byte(`{"unit":"globular-rbac.service"}`),
	}
	if incidentFingerprint(withUnit) == incidentFingerprint(other) {
		t.Error("different units must produce different fingerprints")
	}
	// Same unit, different occurrence → same fingerprint.
	repeat := &ai_executorpb.ProcessIncidentRequest{
		IncidentId:       "i3",
		RuleId:           "service-crash",
		TriggerEventName: "service.exited",
		TriggerEventData: []byte(`{"unit":"globular-repository.service"}`),
	}
	if incidentFingerprint(withUnit) != incidentFingerprint(repeat) {
		t.Error("same unit across occurrences must share a fingerprint")
	}
}
