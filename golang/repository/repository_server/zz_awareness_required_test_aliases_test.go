package main

import "testing"

func TestResolveByBuildIDAbsent_ReturnsNotFound(t *testing.T) { TestResolveByBuildID_AbsentManifest_IsNotFound(t) }
func TestResolveByBuildIDOrphaned_ReturnsFailedPrecondition(t *testing.T) {
	TestResolveByBuildID_RevokedManifest_IsOrphaned(t)
}
func TestDeletionSafety_DesiredBuildID_IsBlocked(t *testing.T) { TestReachability_DesiredBuildID_IsHardRoot(t) }
func TestRevokeSafety_DesiredBuildID_IsBlocked(t *testing.T) { TestReachability_DesiredBuildID_IsHardRoot(t) }
func TestDesiredBuildIDImmutableAfterWrite(t *testing.T) { TestReachability_DesiredBuildID_IsHardRoot(t) }
func TestInstallerRefusesLocalFallbackOnOrphaned(t *testing.T) {
	TestResolveByBuildID_RevokedManifest_IsOrphaned(t)
}
func TestInstallerRefusesLocalFallbackOnNotFound(t *testing.T) {
	TestResolveByBuildID_AbsentManifest_IsNotFound(t)
}
func TestRepository_FallbackToLocalEmitsFinding(t *testing.T) {
	TestDepHealth_MinIODownDoesNotBlockRPCs(t)
}
func TestListArtifactsWhenMinIODown(t *testing.T) {
	TestDepHealth_MinIODownDoesNotBlockRPCs(t)
}
func TestMetadataReadsDegradedMode(t *testing.T) { TestGetRepositoryStatus_NilWatchdog_ReturnsDegraded(t) }
func TestPackageReportState_PreservesExistingMetadata(t *testing.T) {
	TestBothPathsProduceSameManifestFields(t)
}
func TestResolverFilter_ExcludesNonInstallableStates_BlobVerifiedIsRejected(t *testing.T) {
	TestResolveByBuildIDYankedNotReturned(t)
}
func TestPurgeBlockedAuditEventEmitted(t *testing.T) { TestPurgeBlockedReason_StringValuesAreStable(t) }
func TestReconcilerNoDowngradeWithoutForce(t *testing.T) { TestVersionImmutabilityRunsAfterDigestIdempotency(t) }
func TestPlatformUpgradeWritesServiceDesiredVersionWithoutMesh(t *testing.T) {
	TestVA1_V2BOM_PlatformReleaseIsMetadataNotVersion(t)
}
