package main

import "testing"

func TestRepairActionsRequireExplicitIntent(t *testing.T) {
	TestRepairArtifact_RevokedNotRepaired(t)
}

func TestSchemaMigrationsIdempotent(t *testing.T) {
	TestUploadIdempotentSameDigestReturnsExistingBuildID(t)
}
