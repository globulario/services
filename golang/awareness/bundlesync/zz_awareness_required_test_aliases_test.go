package bundlesync

import "testing"

func TestArchitectureClassificationBlockedByStaleBundle(t *testing.T) {
	TestFreshnessStaleOnBuildIDDrift(t)
}
func TestBootstrapRecoveryWithLocalBOMCache(t *testing.T) { TestEnsureLocalCacheFastPath(t) }
func TestAwarenessBuildCleanRemovesOldDB(t *testing.T) { TestInstallBundleRecoversFromCrashAfterRename(t) }
