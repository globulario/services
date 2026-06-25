package config

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestEveryCriticalKeyOwnerGuardedAtThePrimitive is the RT-3 funnel ratchet.
//
// It asserts that EVERY key in CriticalKeyPolicies is owner-guarded by the write
// primitives: a writer that is neither the owner nor an authorized writer is
// rejected on PutRuntimeWithClass, DeleteRuntimeWithClass, AND RunTxnWithClass —
// before any etcd I/O.
//
// This is the bypass ratchet's enforceable half, and it auto-extends:
//   - add a new critical key to the table → it is covered here with no edit;
//   - remove the owner guard from any write primitive → every case here fails.
//
// The other half — catching a NEW raw cli.Put/Txn that bypasses the primitives
// entirely — is the principle-check scanner (raw writes in the swept dirs are
// DRIFT unless consciously allowlisted). Together they close the funnel: critical
// writes must go through the guarded seam, and the seam always guards them.
func TestEveryCriticalKeyOwnerGuardedAtThePrimitive(t *testing.T) {
	kv := &fakeKV{}
	restore := SetWriteKVForTest(kv)
	defer restore()
	txnRestore := SetTxnRunnerForTest(func(context.Context, []clientv3.Op) error { return nil })
	defer txnRestore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	if len(CriticalKeyPolicies) == 0 {
		t.Fatal("CriticalKeyPolicies is empty — the ratchet would vacuously pass")
	}

	// A writer that owns nothing and is authorized nowhere in the table.
	SetLocalWriterIdentity("rogue-not-a-registered-owner")

	for _, p := range CriticalKeyPolicies {
		key := p.Key
		if p.IsPrefix {
			key = p.Key + "ratchet-probe"
		}

		if err := PutRuntimeWithClass(context.Background(), key, []byte("x"), CriticalWrite); err == nil {
			t.Errorf("PutRuntimeWithClass: critical key %q accepted a non-owner write — owner guard missing", key)
		}
		if _, err := DeleteRuntimeWithClass(context.Background(), key, CriticalWrite); err == nil {
			t.Errorf("DeleteRuntimeWithClass: critical key %q accepted a non-owner delete — owner guard missing", key)
		}
		if err := RunTxnWithClass(context.Background(), CriticalWrite, PutOp(key, []byte("x"))); err == nil {
			t.Errorf("RunTxnWithClass: critical key %q accepted a non-owner txn op — owner guard missing", key)
		}
	}

	// The rejected writes must never have reached the KV (guard runs before I/O).
	if kv.callCount() != 0 {
		t.Errorf("a rejected write reached the KV: %d Put(s) recorded", kv.callCount())
	}
}
