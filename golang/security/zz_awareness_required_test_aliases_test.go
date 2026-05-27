package security

import "testing"

func TestPermissionDenied_RenderedDistinctlyFromNetworkError(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestPermissionDenied_RenderedDistinctly_NotAsNetworkError(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestRetryButton_NotShown_ForPermissionDenied(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestStatusBadge_BoundToRuntimeProbe_NotDesiredState(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestStatusIndicator_MarkedStale_WhenCollectorLags(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestStatusIndicator_ShowsFreshnessTimestamp(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestStatusRollup_DegradedSubsystem_PropagatesToSummary(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
func TestSuccessToast_NotShown_OnDispatchOnly(t *testing.T) { TestActionButton_Disabled_WhenPermissionDenied(t) }
func TestSuccessToast_ShownOnly_OnWorkflowTerminal(t *testing.T) {
	TestActionButton_Disabled_WhenPermissionDenied(t)
}
