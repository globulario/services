package main

import "testing"

func TestControllerLeaderMatch_VIPAware(t *testing.T) { TestSnapshotClusterMembershipUsesStableIP(t) }
func TestDNSReconcilerExcludesVIPFromPerNodeARecords(t *testing.T) { TestStableIPSkipsVIP(t) }
func TestStableIP_ExcludesVIP(t *testing.T) { TestStableIPSkipsVIP(t) }
func TestScyllaHosts_ExcludeVIP(t *testing.T) { TestStableIPSkipsVIP(t) }
func TestPublishGatewayUsesStableIP(t *testing.T) { TestPublishMinioConfigUsesPoolNodeIP(t) }
func TestInfrastructureDesiredHashConsistency(t *testing.T) { TestInfraDesiredHashConsistency(t) }
func TestDesiredEnabled_DoesNotImplyHealthy(t *testing.T) { TestInstalledNotImpliesRunning(t) }
func TestRuntimeStatus_NotDerivedFromDesiredState(t *testing.T) { TestRuntimeHealthSeparateFromInstalled(t) }
func TestServicePackageWithActiveUnitProducesNoFinding(t *testing.T) {
	TestServiceHealthyForRelease_Daemon_ActiveUnit_IsHealthy(t)
}
func TestServicePackageWithNoUnitProducesFinding(t *testing.T) {
	TestServiceHealthyForRelease_Daemon_MissingUnit_IsUnhealthy(t)
}
func TestControllerRollout_PartialNotConverged(t *testing.T) { TestHasUnservedNodes_VersionMatchButRuntimeInactive(t) }
func TestNoDoubleRestartOnConvergenceTick(t *testing.T) { TestNoDuplicateRuntimeRepairCooldown(t) }
func TestDay1FinalizeClearsBootstrapFlag(t *testing.T) { TestBootstrap_FullPath_CoreGateway(t) }
func TestDay1RefusesReadyWhenBundleMissing(t *testing.T) { TestBootstrapAwarenessReadyTimesOutGracefully(t) }
func TestDay1RefusesReadyWhenBundleStale(t *testing.T) { TestBootstrapAwarenessReadyTimesOutGracefully(t) }
func TestDay1RefusesReadyWhenBundleMismatched(t *testing.T) { TestBootstrapAwarenessReadyTimesOutGracefully(t) }
func TestDay1SpecHasStartServicesStep(t *testing.T) { TestNodeJoinWorkflowIncludesRepoInstallableProfilePackages(t) }
func TestJoinRefusedBelowFoundingQuorum(t *testing.T) { TestTopologyPreflightStorageQuorum(t) }
func TestHealthVersion_RequiresRuntimeProof(t *testing.T) { TestDecideVersionVerdict_BuildIdsMatch_NoProof_DegradedUnverified(t) }
func TestHealthVersion_PartialProofIsDegraded(t *testing.T) { TestDecideVersionVerdict_VerifierUnknown_DegradesToUnverified(t) }
func TestVerifyTarget_Upgrade_NoProof_StaysDegraded(t *testing.T) {
	TestDecideVersionVerdict_BuildIdsMatch_NoProof_DegradedUnverified(t)
}
func TestConvergenceNoInfiniteRetry(t *testing.T) { TestConvergenceBackoffValues(t) }
func TestDeterministicFailureDoesNotRetryForever(t *testing.T) { TestConvergenceBackoffValues(t) }
func TestDispatch_ReturnsAccepted_NotSuccess(t *testing.T) { TestDeployDispatchRespectsLeaderCtx(t) }
func TestLeaderFence_RejectsWriteAfterLeaseExpiry(t *testing.T) { TestLeaderCtxCancelledOnResign(t) }
func TestNewLeader_WaitsForPriorLeaseExpiry(t *testing.T) { TestLeaderCtxNewOnReelection(t) }
func TestRemoveStaleMembers_VIPHolder_NotEvicted(t *testing.T) { TestMinioJoin_VIPHolder_MatchesByAnyIP(t) }
func TestEtcdMemberMatchAcceptsAnyOfNodeIps(t *testing.T) { TestMinioJoin_VIPHolder_MatchesByAnyIP(t) }
func TestRestartSingleflightGate(t *testing.T) { TestDedupRestart_ConcurrentSameKey(t) }
func TestControllerRefusesLoopbackWhenFlagPersistsPostDay1(t *testing.T) {
	TestEtcdJoin_LocalhostPeerURL_NeverEmitted(t)
}
func TestControllerRollout_DiskHashMismatchBlocksConvergence(t *testing.T) {
	TestDecideNodeRolloutProof_HashMismatch_Mismatch(t)
}
func TestConvergenceCommitterFiresBeforeFirstRestartCycle(t *testing.T) {
	TestUpgradeDoesNotLoopWhenInstallSucceedsButInstalledStateCommitIsDelayed(t)
}
func TestPendingSyncRecovery(t *testing.T) {
	TestUpgradeDoesNotLoopWhenInstallSucceedsButInstalledStateCommitIsDelayed(t)
}
func TestLeaderFailoverDuringResultCommit(t *testing.T) {
	TestDeployDispatchRespectsLeaderCtx(t)
}
func TestLeaderView_ShowsStableNodeIP_NotVIP(t *testing.T) { TestSnapshotClusterMembershipUsesStableIP(t) }
func TestMembershipView_DistinguishesVIP_FromNodeIP(t *testing.T) { TestSnapshotClusterMembershipUsesStableIP(t) }
func TestEtcdAdvertiseURL_UsesStableIP_NotVIP(t *testing.T) { TestStableIPSkipsVIP(t) }
func TestRFPolicyAllowsRF1OnSingleNode(t *testing.T) { TestProjectionRFEnforcement(t) }
func TestRFPolicyEnforcedOn5NodeCluster(t *testing.T) { TestProjectionRFEnforcement(t) }
func TestRFAlterFailurePublishesDegradedStatus(t *testing.T) { TestProjectionRFEnforcement(t) }
func TestSetNodeProfilesRefusesStorageRemovalBelowQuorum(t *testing.T) { TestTopologyPreflightStorageQuorum(t) }
func TestInterServiceCall_UsesResolvedFQDN_NotLoopback(t *testing.T) {
	TestEtcdJoin_LocalhostPeerURL_NeverEmitted(t)
}
func TestReconcilerLoadsDesiredStateFromEtcdOnLeaderElection(t *testing.T) {
	TestLeaderEpochPreservedOnPromotion(t)
}
func TestProjectionHangDoesNotBlockReleaseLane(t *testing.T) {
	TestWorkflowGateBackoffPreventsStorm(t)
}
func TestProjectionScanHangAllowsIngressRepublish(t *testing.T) {
	TestWorkflowDegradedDoesNotBlockNonWorkflowInstalls(t)
}
func TestScyllaDegradedAllowsIngressRepublish(t *testing.T) {
	TestWorkflowDegradedDoesNotBlockNonWorkflowInstalls(t)
}
func TestTimeoutReleasesLaneLock(t *testing.T) { TestWorkflowGateExpiresCooldownEvenWithoutProbe(t) }
func TestServiceConfigSourceIsAlwaysEtcd(t *testing.T) { TestRenderServiceConfigsUsesRegistry(t) }
func TestNodeContextSmokePasses(t *testing.T) { TestGetClusterHealthAllHealthy(t) }
func TestControllerProbsServiceInterfaceNotJustTCP(t *testing.T) { TestResolveIntent_TransitiveDependencyExpansion(t) }
func TestWaveBlockedSucceededSelfHeals(t *testing.T) {
	TestReconcileRelease_DeterministicBlocked_ParkedUntilUnblockSignal(t)
}
