package config

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestRunTxnWithClass_CommitsAllOps proves a guarded transaction commits every op
// atomically through a single Txn when the writer is authorized.
func TestRunTxnWithClass_CommitsAllOps(t *testing.T) {
	var got []clientv3.Op
	restore := SetTxnRunnerForTest(func(_ context.Context, ops []clientv3.Op) error {
		got = ops
		return nil
	})
	defer restore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	// cluster-controller owns ingress keys → authorized.
	SetLocalWriterIdentity("cluster-controller")
	err := RunTxnWithClass(context.Background(), CriticalWrite,
		PutOp("/globular/ingress/v1/spec", []byte("a")),
		PutOp("/globular/ingress/v1/spec_backup", []byte("a")),
	)
	if err != nil {
		t.Fatalf("RunTxnWithClass: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 ops committed in one txn, got %d", len(got))
	}
}

// TestRunTxnWithClass_GuardRejectsOffOwnerKey proves an off-owner key rejects the
// WHOLE transaction before any op is committed (all-or-nothing guard).
func TestRunTxnWithClass_GuardRejectsOffOwnerKey(t *testing.T) {
	committed := false
	restore := SetTxnRunnerForTest(func(_ context.Context, _ []clientv3.Op) error {
		committed = true
		return nil
	})
	defer restore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	// node-agent is NOT an authorized writer of /globular/ingress (controller-owned).
	SetLocalWriterIdentity("node-agent")
	err := RunTxnWithClass(context.Background(), CriticalWrite,
		PutOp("/globular/nodes/n1/x", []byte("ok")),            // node-agent owns this
		PutOp("/globular/ingress/v1/spec", []byte("rejected")), // ...but not this
	)
	if err == nil {
		t.Fatal("expected the transaction to be rejected: node-agent cannot write ingress")
	}
	if committed {
		t.Fatal("transaction must NOT commit when any key fails the owner guard")
	}
}

// TestRunTxnWithClass_FailOpenUnregistered proves an unregistered identity is
// unguarded (consistent with PutRuntimeWithClass), so tests/tools still work.
func TestRunTxnWithClass_FailOpenUnregistered(t *testing.T) {
	committed := false
	restore := SetTxnRunnerForTest(func(_ context.Context, _ []clientv3.Op) error {
		committed = true
		return nil
	})
	defer restore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	SetLocalWriterIdentity("")
	if err := RunTxnWithClass(context.Background(), CriticalWrite,
		PutOp("/globular/ingress/v1/spec", []byte("x"))); err != nil {
		t.Fatalf("unregistered identity should be unguarded: %v", err)
	}
	if !committed {
		t.Fatal("expected the transaction to commit under fail-open")
	}
}
