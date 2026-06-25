package config

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// write_txn.go — the transaction-shaped counterpart of PutRuntimeWithClass.
//
// Some owner-owned writes must be atomic across several keys (e.g. the ingress
// spec and its backup must move together, or neither). A bare clientv3 Txn does
// that but bypasses the owner-ownership guard the single-key primitive applies
// (RT-3). RunTxnWithClass closes that gap: it guards EVERY key in the transaction
// against the registered writer identity, then commits all ops in one Txn under
// the WriteClass retry/timeout policy. All-or-nothing.

// TxnOp is one operation in a guarded transaction — a Put when Value is non-nil,
// a Delete otherwise. Build them with PutOp / DeleteOp.
type TxnOp struct {
	Key    string
	Value  []byte
	delete bool
}

// PutOp is a put of value at key within a guarded transaction.
func PutOp(key string, value []byte) TxnOp { return TxnOp{Key: key, Value: value} }

// DeleteOp is a delete of key within a guarded transaction.
func DeleteOp(key string) TxnOp { return TxnOp{Key: key, delete: true} }

// txnRunnerOverride lets tests observe the committed ops without a live etcd.
var txnRunnerOverride func(ctx context.Context, ops []clientv3.Op) error

// SetTxnRunnerForTest installs a transaction runner override and returns a
// restore func. Test-only.
func SetTxnRunnerForTest(fn func(ctx context.Context, ops []clientv3.Op) error) func() {
	writeKVMu.Lock()
	prev := txnRunnerOverride
	txnRunnerOverride = fn
	writeKVMu.Unlock()
	return func() {
		writeKVMu.Lock()
		txnRunnerOverride = prev
		writeKVMu.Unlock()
	}
}

func runTxnOps(ctx context.Context, ops []clientv3.Op) error {
	writeKVMu.Lock()
	ov := txnRunnerOverride
	writeKVMu.Unlock()
	if ov != nil {
		return ov(ctx, ops)
	}
	cli, err := etcdClient()
	if err != nil {
		return err
	}
	_, err = cli.Txn(ctx).Then(ops...).Commit()
	return err
}

// RunTxnWithClass commits all ops in a single etcd transaction (atomic: all or
// none), after applying the owner-ownership guard to every key for the registered
// writer identity. A guard failure on any key rejects the whole transaction
// before anything is written. Fail-open when no identity is registered (same as
// PutRuntimeWithClass). Empty ops is a no-op.
func RunTxnWithClass(ctx context.Context, class WriteClass, ops ...TxnOp) error {
	if len(ops) == 0 {
		return nil
	}

	// Guard every key first — a partial guard would defeat the all-or-nothing
	// contract (one off-owner key must reject the entire transaction).
	for _, op := range ops {
		if err := guardLocalWriterOwnership(op.Key); err != nil {
			return fmt.Errorf("RunTxnWithClass(%s): %w", class, err)
		}
	}

	cops := make([]clientv3.Op, 0, len(ops))
	for _, op := range ops {
		if op.delete {
			cops = append(cops, clientv3.OpDelete(op.Key))
		} else {
			cops = append(cops, clientv3.OpPut(op.Key, string(op.Value)))
		}
	}

	policy := GetWritePolicy(class)
	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("RunTxnWithClass(%s): context done after %d attempt(s): %w (last: %v)",
					class, attempt, ctx.Err(), lastErr)
			}
			return fmt.Errorf("RunTxnWithClass(%s): context done before first attempt: %w", class, ctx.Err())
		default:
		}

		tctx, cancel := context.WithTimeout(ctx, policy.Timeout)
		err := runTxnOps(tctx, cops)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt < policy.MaxRetries {
			sleep := policy.BaseBackoff
			if policy.Jitter > 0 && sleep > 0 {
				sleep += time.Duration(float64(sleep) * policy.Jitter * writeJitter())
			}
			if sleep > 0 {
				select {
				case <-ctx.Done():
					return fmt.Errorf("RunTxnWithClass(%s): context cancelled during backoff: %w (last: %v)",
						class, ctx.Err(), lastErr)
				case <-time.After(sleep):
				}
			}
		}
	}
	return fmt.Errorf("RunTxnWithClass(%s): etcd txn after %d attempt(s): %w",
		class, policy.MaxRetries+1, lastErr)
}
