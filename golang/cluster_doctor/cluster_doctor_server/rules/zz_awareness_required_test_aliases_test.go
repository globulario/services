package rules

import "testing"

func TestDoctorRule_RepositoryDesiredBuildIDsResolve(t *testing.T) {
	TestDoctorRule_RepositoryDesiredBuildIDsResolve_OrphanFires(t)
}
func TestFingerprintDivergence_DataIncomplete_SuppressesMissing(t *testing.T) {
	TestFingerprintDivergence_MissingOnly_DataIncomplete_NoFinding(t)
}
func TestDataIncomplete_SuppressesCritical_WhenNoKnownBad(t *testing.T) {
	TestWriteQuorumLost_DataIncomplete_NoCritical(t)
}
func TestPartialSnapshot_NoCritical_WhenOnlyUnknown(t *testing.T) {
	TestWriteQuorumLost_PartialSnapshot_NoFalsePositive(t)
}
func TestDoctorRule_DNSRecordsMatchRuntimeHealth(t *testing.T) {
	TestDoctorRule_DNSRecordsMatchRuntimeHealth_InstalledButInactive(t)
}
func TestDriftOver5MinIsError(t *testing.T) { TestClusterServicesDrift_LongAge_IsError(t) }
func TestDriftUnder2MinIsWarn(t *testing.T) { TestClusterServicesDrift_ShortAge_IsWarn(t) }
func TestDriftOver5MinOnCriticalServiceIsCritical(t *testing.T) {
	TestClusterServicesDrift_LongAge_IsError(t)
}
func TestExpiredCache_TreatedAsUnknown_NotHealthy(t *testing.T) {
	TestCollectorGap_ProducesUnknown_NotHealthy(t)
}
func TestObjectstoreAdmit_ExistingData_RequiresForceFlag(t *testing.T) {
	TestExistingDataGuard_ExistingDataNotAcknowledged_CRITICAL(t)
}
func TestObjectstoreTopologyWipe_BlockedByDataGuard(t *testing.T) {
	TestExistingDataGuard_ExistingDataNotAcknowledged_CRITICAL(t)
}
func TestObjectstoreNoDesiredStateDoesNotFireWithNoStorageNodes(t *testing.T) {
	TestDoctorNoDesiredState_NoNodes(t)
}
func TestObjectstoreNoDesiredStateFiresWhenNilAndStorageNodes(t *testing.T) {
	TestDoctorNoDesiredState_WithStorageNodes(t)
}
func TestObjectstoreQuorum_AbsentNodeIsUnknownNotDown(t *testing.T) {
	TestAbsentRecord_ClassifiedAsUnknown_NotKnownBad(t)
}
func TestObjectstoreQuorum_KnownDown_StillCountsForLoss(t *testing.T) {
	TestWriteQuorumLost_KnownDown_DataIncomplete_StillFires(t)
}
func TestObjectstoreQuorum_PartialSnapshot_EmitsCaveat(t *testing.T) {
	TestWriteQuorumLost_PartialSnapshot_LocalNodeOnly_Suppressed(t)
}
func TestPKICANotPublishedClearsWhenCAPresent(t *testing.T) { TestPKICANotPublished_CAPresent_NoFinding(t) }
func TestPKICANotPublishedFiresWhenCAMetadataNilAndNodesExist(t *testing.T) {
	TestPKICANotPublished_CAMissing_WithNodes_FindingFired(t)
}
func TestDoctorFinding_ExistingDataGuard_CriticalWhenNotForceAcknowledged(t *testing.T) {
	TestExistingDataGuard_ExistingDataNotAcknowledged_CRITICAL(t)
}
func TestDoctorEmitsDriftFindingButDoesNotGate(t *testing.T) { TestClusterServicesDrift_HashMismatch(t) }
func TestDoctorDetectsSeedDriftWithinOneCycle(t *testing.T) { TestOpsKnowledgeSeedIntegrity_DriftedHash_Error(t) }
func TestPublishedServiceWithoutDesiredStateDetectedByDoctor(t *testing.T) {
	TestDoctorRule_DNSRecordsMatchRuntimeHealth_PlannedNotInstalled(t)
}
func TestStatusRollup_UnknownSubsystem_NotGreen(t *testing.T) { TestCollectorGap_ProducesUnknown_NotHealthy(t) }
func TestMissingKeyDoesNotStopRuntime(t *testing.T) { TestMalformedDisableDoesNotStopRuntime(t) }
func TestServiceLiveness_UsesRuntimeProbe_NotDesiredState(t *testing.T) {
	TestInstalledStateRuntimeMismatch_DaemonMissingUnit_FindingFired(t)
}
func TestObjectstoreRegistryEntryComplete(t *testing.T) { TestCriticalKeyRegistryPresence_PresentKeysNoFinding(t) }
func TestPKIRegistryEntryComplete(t *testing.T) { TestCriticalKeyRegistryPresence_PresentKeysNoFinding(t) }
func TestPackageKindFromCanonicalRegistry(t *testing.T) { TestPackageKindMismatch_Empty(t) }
func TestNormalizerEmitsScyllaCQLUnreachableOnFailedUnit(t *testing.T) {
	TestNativeDependencyMissing_SQLFailedUnit_FindingFired(t)
}
func TestDeleteKeyWhileRunningKeepsRuntimeActive(t *testing.T) { TestMalformedDisableDoesNotStopRuntime(t) }
func TestUnauthorizedAttemptIsLoggedAndRejected(t *testing.T) { TestNodeReachable_StableFindingID(t) }
func TestUnhealthyMasterLosesPriorityBelowBackup(t *testing.T) {
	TestWriteQuorumLost_VIPHolder_RealDowntimeStillFires(t)
}
func TestDestructiveAction_RequiresConfirmation(t *testing.T) {
	TestDestructiveGuard_FingerprintChange_UnapprovedTransition_Warn(t)
}
func TestDestructiveWarning_NotHiddenByThemeChange(t *testing.T) {
	TestDestructiveGuard_FingerprintChange_UnapprovedTransition_Warn(t)
}
func TestDestructiveWarning_VisibleAtAllBreakpoints(t *testing.T) {
	TestDestructiveGuard_FingerprintChange_UnapprovedTransition_Warn(t)
}
func TestDiscoveredDependenciesAreDeclaredOrClassified(t *testing.T) {
	TestPackageKindMismatch_MultipleRecords(t)
}
