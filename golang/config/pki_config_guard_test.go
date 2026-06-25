package config

import (
	"context"
	"testing"
)

// TestSaveCAMetadata_GuardedAndOwnerEnforced proves the RT-3 funnel migration of
// the /globular/pki/ca write: it routes through the governed critical-write seam,
// so the owner-guard applies — the controller may write CA metadata, a non-owner
// may not.
func TestSaveCAMetadata_GuardedAndOwnerEnforced(t *testing.T) {
	kv := &fakeKV{}
	restore := SetWriteKVForTest(kv)
	defer restore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	meta := CAMetadata{Fingerprint: "sha256:abc", Generation: 1, Active: true}

	// cluster-controller owns /globular/pki/ca → allowed, routed through the guard.
	SetLocalWriterIdentity("cluster-controller")
	if err := SaveCAMetadata(context.Background(), meta); err != nil {
		t.Fatalf("controller CA-metadata write should succeed: %v", err)
	}
	if kv.callCount() != 1 {
		t.Errorf("expected the write to route through the guarded primitive (1 Put), got %d", kv.callCount())
	}

	// node-agent is not an authorized writer of the controller-owned CA key.
	SetLocalWriterIdentity("node-agent")
	if err := SaveCAMetadata(context.Background(), meta); err == nil {
		t.Fatal("expected node-agent write to /globular/pki/ca to be rejected by the owner guard")
	}
	if kv.callCount() != 1 {
		t.Errorf("rejected write must not reach etcd: Put count moved to %d", kv.callCount())
	}
}
