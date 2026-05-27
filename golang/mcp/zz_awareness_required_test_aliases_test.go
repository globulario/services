package main

import "testing"

func TestAwarenessBundleMissingClassificationOnFirstJoin(t *testing.T) { TestFreshnessStatusMissingManifest(t) }
func TestDeleteRejectsParentTraversal(t *testing.T) { TestAwarenessProposeFromIncident_RejectsPathTraversal(t) }
func TestDeleteRejectsRootAndEmptyPath(t *testing.T) { TestFilePathValidationEmptyRoots(t) }
func TestPathNormalizationRejectsTraversal(t *testing.T) { TestAwarenessProposeFromIncident_RejectsPathTraversal(t) }
func TestPathNormalizationRejectsEncodedTraversal(t *testing.T) { TestAwarenessProposeFromIncident_RejectsPathTraversal(t) }
func TestPathNormalizationKeepsUserRootBoundary(t *testing.T) { TestFilePathValidation(t) }
func TestDay1ClassifyNodeMCPToolReturnsBLOCKForScyllaDown(t *testing.T) { TestAwarenessReadiness_WritablePathsAdvertiseAllTools(t) }
func TestExplainImpactByFile_ReturnsMissingLinks(t *testing.T) { TestFileImpact_DeclaredEdgesLabeledLowerConfidence(t) }
func TestExplainImpactByFile_MandatoryForbiddenFix(t *testing.T) { TestFileImpact_MaxDepthRespected(t) }
func TestOfflineDiagnose_EtcdLeaderLoss_MapsToEtcdLeaderInstability(t *testing.T) {
	TestOfflineDiagnose_EtcdNOSPACE_MapsToEtcdFailureMode(t)
}
func TestSuggestIncident_ControlPlaneCascade_EtcdFirst(t *testing.T) {
	TestCausalChain_EtcdNOSPACE_ProducesControlPlaneCascade(t)
}
func TestSourceRoot_ExplicitPathInvalidReturnsInaccessible(t *testing.T) {
	TestCoverageReport_NoRepoRootIsUnverifiedNotCritical(t)
}
func TestSourceRoot_NotInGitRepoReturnsAbsent(t *testing.T) {
	TestCoverageReport_NoRepoRootIsUnverifiedNotCritical(t)
}
func TestSemanticPathIncludesExplanation(t *testing.T) {
	TestAwarenessFindingContext_ReturnsStructuredTrace(t)
}
func TestSelfCheckReportsNoisySections(t *testing.T) { TestCoverageReport_UnverifiedImplementedGap(t) }
func TestOfflineDiagnose_ControllerLeaseExpired_MapsToCorrectFailureMode(t *testing.T) {
	TestOfflineDiagnose_EtcdNOSPACE_MapsToEtcdFailureMode(t)
}
func TestOfflineDiagnose_WorkflowTimeout_MapsToControlPlaneInstability(t *testing.T) {
	TestOfflineDiagnose_EtcdNOSPACE_DoesNotMapToObjectstore(t)
}
func TestMCPLoadsBundlePathFirst(t *testing.T) { TestAwarenessBundleManifestPresent(t) }
func TestMCPStartFailsWhenOrphanHoldsPort(t *testing.T) { TestRuntimeActivationCheck_MissingTLSFiles(t) }
func TestMCPStartedAfterDay1Install(t *testing.T) { TestAwarenessBundleToolsAreAllowlistedForAggregator(t) }
func TestGraphDriftNoStaleRefs(t *testing.T) { TestTrustFiltering_StaleEdgeReported(t) }
