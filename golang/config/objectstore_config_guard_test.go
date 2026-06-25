package config

import (
	"context"
	"testing"
)

// TestSaveObjectStoreDesiredState_GuardedAndOwnerEnforced proves the RT-3 funnel
// migration of the /globular/objectstore/config write: it routes through the
// governed critical-write seam (so the owner-guard applies) — the controller may
// write it, a non-owner may not.
func TestSaveObjectStoreDesiredState_GuardedAndOwnerEnforced(t *testing.T) {
	kv := &fakeKV{}
	restore := SetWriteKVForTest(kv)
	defer restore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	state := &ObjectStoreDesiredState{} // empty endpoint is a valid (degraded) state

	// cluster-controller owns /globular/objectstore/config → allowed, and the write
	// goes through the guarded primitive (recorded by the fake KV).
	SetLocalWriterIdentity("cluster-controller")
	if err := SaveObjectStoreDesiredState(context.Background(), state); err != nil {
		t.Fatalf("controller write should succeed: %v", err)
	}
	if kv.callCount() != 1 {
		t.Errorf("expected the write to route through the guarded primitive (1 Put), got %d", kv.callCount())
	}

	// node-agent is not an authorized writer of this controller-owned key → rejected
	// before any etcd write.
	SetLocalWriterIdentity("node-agent")
	if err := SaveObjectStoreDesiredState(context.Background(), state); err == nil {
		t.Fatal("expected node-agent write to /globular/objectstore/config to be rejected by the owner guard")
	}
	if kv.callCount() != 1 {
		t.Errorf("rejected write must not reach etcd: Put count moved to %d", kv.callCount())
	}
}
