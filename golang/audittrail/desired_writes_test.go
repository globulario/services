package audittrail

import "testing"

func TestValidateDesiredWriteRecord_RequiredFields(t *testing.T) {
	ok := DesiredWriteRecord{
		Service:   "cluster-controller",
		Actor:     "cluster-controller",
		Source:    "reconcileDesiredFromRepository",
		Action:    "update_desired_build",
		Reason:    "published build advanced",
		Timestamp: "2026-05-26T00:00:00Z",
	}
	if err := validateDesiredWriteRecord(ok); err != nil {
		t.Fatalf("expected valid record, got err: %v", err)
	}

	bad := ok
	bad.Actor = ""
	if err := validateDesiredWriteRecord(bad); err == nil {
		t.Fatal("expected error for missing actor")
	}
}
