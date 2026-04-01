package main

import "testing"

// testPlanSigner creates a planSigner for test servers.
// The plan signing system is removed; this returns an empty struct.
func testPlanSigner(t *testing.T) *planSigner {
	t.Helper()
	return &planSigner{}
}
