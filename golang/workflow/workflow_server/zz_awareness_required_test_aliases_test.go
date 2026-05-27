package main

import "testing"

func TestDrainOnStartupCatchesQueuedRequest(t *testing.T) { TestDispatchOrderAbandonedBeforeCooldown(t) }
func TestDerivedLaneErrorDoesNotFlipAuthorityToBlocked(t *testing.T) { TestWorkflowDispatch_ReturnsAccepted_NotSuccess(t) }
func TestServiceReleaseRecoveryAfterWorkflowCommitFailure(t *testing.T) { TestSchedulerResumesAfterCooldown(t) }
