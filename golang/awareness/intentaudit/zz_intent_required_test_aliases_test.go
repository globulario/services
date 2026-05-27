package intentaudit

import "testing"

func TestIntentSchemaValidation(t *testing.T) {
	TestLoadDir_WithNewMetadata(t)
}

func TestRuntimeEvidenceFreshnessGate(t *testing.T) {
	TestRuntimeObservationDoesNotMutateDesired_Pass(t)
}
