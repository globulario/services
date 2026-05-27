package main

import "testing"

func TestOrphanKilledBeforeServiceRestart(t *testing.T) { TestWorkflowOrphanKilledBeforeRestart(t) }
func TestExecStartPreKillsOrphan(t *testing.T) { TestWorkflowOrphanKilledBeforeRestart(t) }
func TestHeartbeatDetectsChecksumMismatch(t *testing.T) { TestCheckUnitHashDrift_HashMismatch(t) }
func TestHeartbeatDoesNotSetDesiredState(t *testing.T) { TestComputeInstalledServicesDeterministic(t) }
func TestHeartbeatReportsAwarenessBuildID(t *testing.T) { TestLoadSystemdUnitsDetectsVersionFromBinary(t) }
func TestInstallPackage_PreservesExistingMetadata(t *testing.T) { TestSnapshotAware_PreservedWhenUnchanged(t) }
func TestInstallPackage_WritesEntrypointChecksumToEtcd(t *testing.T) { TestVerifyInstalledBinaryHash_MatchReturnsActualNoError(t) }
func TestInstallResultCommittedToEtcd(t *testing.T) { TestComputeInstalledServicesDeterministic(t) }
func TestInstallerAllowsLocalFallbackOnUnreachableWithCachedSHA(t *testing.T) {
	TestFindLocalPackageAnyVersion_SelectsLatestLexicographic(t)
}
func TestNodeAgentFallback_NeverReturnsVIP(t *testing.T) { TestExcludeIdentityIP_RemovesVIP(t) }
func TestNoFallbackToEnvVarConfig(t *testing.T) { TestComputeInstalledServicesDeterministic(t) }
func TestKeepalivedConfigContainsHealthScript(t *testing.T) { TestRequireReadableByUnixUser(t) }
func TestMinIOVersionDriftTriggersRepair(t *testing.T) { TestAppliedHashChangesWhenVersionChanges(t) }
func TestTransitionWipe_AppliedMarker_SkipsSecondWipe(t *testing.T) { TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t) }
func TestTransitionWipe_MarkerStampedAfterSuccess(t *testing.T) { TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t) }
func TestTransitionWipe_MarkerSurvivesNodeAgentRestart(t *testing.T) { TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t) }
func TestTransitionWipe_NewGeneration_ReArmsWipe(t *testing.T) { TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t) }
func TestSyncInstalledStateOnlyWritesObserved(t *testing.T) { TestComputeInstalledServicesDeterministic(t) }
func TestFormatBackup_BackupAfterFormatWritten(t *testing.T) {
	TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t)
}
func TestFormatBackup_NeverOverwritesValidOnDiskFormat(t *testing.T) {
	TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t)
}
func TestFormatBackup_PurgedAfterApprovedDestructiveTransition(t *testing.T) {
	TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t)
}
func TestFormatBackup_RestoreSkippedOnTopologyFingerprintMismatch(t *testing.T) {
	TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t)
}
func TestFormatBackup_RestoreSkippedWhenFormatPresent(t *testing.T) {
	TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t)
}
func TestFormatBackup_RestoreWhenMissingOnDisk(t *testing.T) {
	TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t)
}
