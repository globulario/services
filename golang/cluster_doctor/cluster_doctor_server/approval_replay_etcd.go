// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor
// @awareness file_role=operator_approval_replay_from_etcd
// @awareness implements=globular.platform:intent.audit.every_authority_change_is_explainable
// @awareness risk=high
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// approvalReplayEtcdPrefix is where single-use approval-token jtis are
// recorded. Each entry is leased for the token's remaining lifetime so
// expired tokens get garbage-collected automatically and the store
// doesn't grow unbounded.
const approvalReplayEtcdPrefix = "/globular/cluster_doctor/approval_replay/"

// etcdReplayStore satisfies security.ReplayStore using etcd as the
// authority. Replay enforcement now survives doctor restart and leader
// failover — both of which would lose the in-memory map. If etcd is
// unreachable, MarkUsed returns an error so the approval is rejected.
// Failing closed is the right call: we cannot prove single-use without
// a durable record, and accepting the token anyway would silently
// degrade the contract.
type etcdReplayStore struct{}

// newEtcdReplayStore returns the production replay store wired to the
// cluster etcd. In tests, see etcdReplayPutFn override.
func newEtcdReplayStore() security.ReplayStore {
	return etcdReplayStore{}
}

// etcdReplayPutFn is the test seam. Production assigns the real etcd
// transaction; tests override to simulate first-use vs. replay vs.
// etcd-unreachable scenarios without spinning up etcd.
var etcdReplayPutFn = etcdReplayPutReal

// MarkUsed implements security.ReplayStore. Atomically reserves jti via
// a CompareAndSwap-style transaction: success → first use; failure →
// jti already recorded → ErrTokenAlreadyUsed.
func (etcdReplayStore) MarkUsed(jti string, expiresAt time.Time) error {
	if jti == "" {
		return errors.New("approval replay: jti is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ttl := time.Until(expiresAt)
	if ttl < time.Second {
		// Token is already past expiry (or within a second). No point
		// leasing — but we still need to record it so a replay attempt
		// within the same second fails. Use a 1s lease floor.
		ttl = time.Second
	}
	return etcdReplayPutFn(ctx, jti, ttl)
}

// etcdReplayPutReal is the production implementation. Uses an etcd Txn
// with Create-revision == 0 to atomically reserve the jti key.
func etcdReplayPutReal(ctx context.Context, jti string, ttl time.Duration) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("approval replay: etcd unavailable, refusing to honor token: %w", err)
	}
	leaseSec := int64(ttl / time.Second)
	if leaseSec < 1 {
		leaseSec = 1
	}
	lease, err := cli.Grant(ctx, leaseSec)
	if err != nil {
		return fmt.Errorf("approval replay: lease grant failed, refusing to honor token: %w", err)
	}
	key := approvalReplayEtcdPrefix + jti
	// Txn: if the key doesn't exist (CreateRevision == 0), put it with
	// the lease. If it already exists, the txn succeeds with !Succeeded
	// and we report replay.
	resp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, "1", clientv3.WithLease(lease.ID))).
		Commit()
	if err != nil {
		return fmt.Errorf("approval replay: txn failed, refusing to honor token: %w", err)
	}
	if !resp.Succeeded {
		// Best-effort lease revoke — we minted it but won't use it. Not
		// fatal if revoke fails; the lease will expire on its own.
		_, _ = cli.Revoke(ctx, lease.ID)
		return security.ErrTokenAlreadyUsed
	}
	return nil
}
