package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// testPlanSigner creates a valid planSigner for test servers.
func testPlanSigner(t *testing.T) *planSigner {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return &planSigner{privateKey: priv, publicKey: pub, kid: "test-kid"}
}
