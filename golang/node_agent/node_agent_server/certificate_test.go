package main

import "testing"

// TestCAKeySync_DisabledByDefaultInClusterMode verifies that CA private key
// sync from MinIO is disabled by default. The CA private key must only reside
// on the signer authority node; distributing it via MinIO requires explicit
// operator opt-in after verifying MinIO health and RBAC requirements.
//
// This test guards against accidental regressions where caKeySyncEnabled is
// flipped to true at package initialization — which would silently spread
// the CA private key to every cluster node.
func TestCAKeySync_DisabledByDefaultInClusterMode(t *testing.T) {
	if caKeySyncEnabled {
		t.Fatal("caKeySyncEnabled must be false by default — " +
			"CA private key must not be synced from MinIO without explicit EnableCAKeySync() opt-in. " +
			"Enabling this without verified MinIO health and RBAC signer role exposes the CA private key to all nodes.")
	}
}
