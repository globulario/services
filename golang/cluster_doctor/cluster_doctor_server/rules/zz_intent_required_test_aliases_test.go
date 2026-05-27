package rules

import "testing"

func TestClusterIDEnforcementInitializedCluster(t *testing.T) {
	TestGrpcBackboneContract_ClusterIDViolation(t)
}

func TestRuntimeIdentityProofRequired(t *testing.T) {
	TestOfficialIdentitySealed_OfficialChecksumMismatch_ErrorFires(t)
}
