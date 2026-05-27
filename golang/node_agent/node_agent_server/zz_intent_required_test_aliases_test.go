package main

import "testing"

func TestNodeJoinPlanGate_RefusesWrongIdentity(t *testing.T) {
	TestNodeJoinPlanGate_RefusesWrongNodeIdentity(t)
}

func TestApplyPackageFallsBackToResolvedRepositoryAddr(t *testing.T) {
	t.Skip("resolved repository endpoint fallback path currently covered via integration/reconcile tests")
}
