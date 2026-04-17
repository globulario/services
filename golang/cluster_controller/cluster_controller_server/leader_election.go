package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/globulario/services/golang/globular_service"
	"github.com/google/uuid"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const leaderElectionPrefix = "/globular/clustercontroller/leader"
const leaderEpochKey = "/globular/clustercontroller/epoch"
const publishAttemptTimeout = 2 * time.Second

var seedRandOnce sync.Once

func seedRand() {
	seedRandOnce.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
}

func startLeaderElection(ctx context.Context, cli *clientv3.Client, srv *server, addr string) {
	seedRand()
	safeGoTracked("leader-election", 15*time.Second, func(h *globular_service.SubsystemHandle) {
		backoff := 250 * time.Millisecond
		maxBackoff := 5 * time.Second
		resetBackoff := func() { backoff = 250 * time.Millisecond }
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ttl := 15 // default lease TTL in seconds
			sess, err := concurrency.NewSession(cli, concurrency.WithTTL(ttl))
			if err != nil {
				log.Printf("leader election: create session failed: %v", err)
				h.TickError(err)
				backoff = sleepWithJitter(backoff, maxBackoff)
				continue
			}
			election := concurrency.NewElection(sess, leaderElectionPrefix)
			host, _ := os.Hostname()
			candidateID := fmt.Sprintf("%s:%d:%s", host, os.Getpid(), uuid.NewString())
			if err := election.Campaign(ctx, candidateID); err != nil {
				log.Printf("leader election: campaign failed: %v", err)
				_ = sess.Close()
				h.TickError(err)
				backoff = sleepWithJitter(backoff, maxBackoff)
				continue
			}
			// Reset backoff after successful campaign
			resetBackoff()
			// Gained leadership — reload authoritative state from etcd before
			// enabling reconciliation, so we pick up state from the previous leader.
			srv.reloadStateFromEtcd()

			// Increment the fencing epoch. This prevents stale leaders (who
			// lost their lease but haven't noticed yet) from making writes
			// that conflict with the new leader.
			epoch := incrementEpoch(ctx, cli)
			srv.leaderEpoch.Store(epoch)
			log.Printf("leader election: epoch incremented to %d", epoch)

			// Seed core workflow definitions to etcd. These must be available
			// before reconciliation starts — they're the workflows the cluster
			// uses to deploy, join, and heal itself.
			srv.seedCoreWorkflowsToEtcd()

			srv.setLeader(true, candidateID, addr)
			h.Tick()
			h.SetMeta("role", "leader")
			if err := publishLeaderAddr(ctx, cli, sess.Lease(), addr); err != nil {
				log.Printf("leader election: publish addr failed: %v", err)
			} else {
				log.Printf("leader election: became leader id=%s addr=%s", candidateID, addr)
				srv.emitClusterEvent("controller.leader_elected", map[string]interface{}{
					"leader_id": candidateID,
					"address":   addr,
				})
			}
			refreshTicker := time.NewTicker(10 * time.Second)
		loop:
			for {
				select {
				case <-ctx.Done():
					cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					_ = election.Resign(cctx)
					cancel()
					break loop
				case <-sess.Done():
					break loop
				case <-srv.resignCh:
					log.Printf("leader election: resign requested via RPC — backing off before re-campaign")
					rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
					_ = election.Resign(rctx)
					rcancel()
					// After an RPC resign (deploy workflow), wait long enough for
					// a follower to win the election before we re-campaign. The
					// workflow still needs this process alive to execute the
					// "apply old leader" step, so we can't block forever.
					refreshTicker.Stop()
					srv.setLeader(false, "", "")
					_ = sess.Close()
					log.Printf("leader election: backing off 60s to let followers campaign first")
					select {
					case <-ctx.Done():
						return
					case <-time.After(60 * time.Second):
						log.Printf("leader election: resign backoff expired, resuming campaign")
					}
					break loop
				case <-refreshTicker.C:
					if err := publishLeaderAddr(ctx, cli, sess.Lease(), addr); err != nil {
						log.Printf("leader election: refresh publish failed: %v", err)
					}
				}
			}
			refreshTicker.Stop()
			srv.setLeader(false, "", "")
			_ = sess.Close()
			backoff = sleepWithJitter(backoff, maxBackoff)
		}
	})
}

func startLeaderWatcher(ctx context.Context, cli *clientv3.Client, srv *server) {
	seedRand()
	if cli == nil {
		return
	}
	safeGo("leader-watcher", func() {
		key := leaderElectionPrefix + "/addr"
		backoff := 250 * time.Millisecond
		maxBackoff := 5 * time.Second
		var rev int64

		syncState := func() {
			if resp, err := cli.Get(ctx, key); err == nil {
				if len(resp.Kvs) > 0 {
					srv.leaderAddr.Store(string(resp.Kvs[0].Value))
				} else {
					srv.leaderAddr.Store("")
				}
				rev = resp.Header.Revision
			}
		}
		syncState()
		wch := cli.Watch(ctx, key, clientv3.WithRev(rev+1))
		for {
			select {
			case <-ctx.Done():
				return
			case wr, ok := <-wch:
				if !ok || wr.Canceled {
					backoff = sleepWithJitter(backoff, maxBackoff)
					wch = cli.Watch(ctx, key, clientv3.WithRev(rev+1))
					continue
				}
				backoff = 250 * time.Millisecond
				if wr.CompactRevision > 0 || wr.Err() == rpctypes.ErrCompacted {
					rev = wr.CompactRevision
					syncState()
					wch = cli.Watch(ctx, key, clientv3.WithRev(rev+1))
					continue
				}
				if wr.Header.GetRevision() > 0 {
					rev = wr.Header.GetRevision()
				}
				for _, ev := range wr.Events {
					switch ev.Type {
					case clientv3.EventTypePut:
						srv.leaderAddr.Store(string(ev.Kv.Value))
					case clientv3.EventTypeDelete:
						srv.leaderAddr.Store("")
					}
				}
			}
		}
	})
}

func publishLeaderAddr(ctx context.Context, cli *clientv3.Client, lease clientv3.LeaseID, addr string) error {
	key := leaderElectionPrefix + "/addr"
	backoff := 100 * time.Millisecond
	maxBackoff := 2 * time.Second
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		timeout := publishAttemptTimeout
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		if remaining < timeout {
			timeout = remaining
		}
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		_, err := cli.Put(attemptCtx, key, addr, clientv3.WithLease(lease))
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		backoff = sleepWithCustomJitter(backoff, maxBackoff, 100*time.Millisecond)
	}
}

func sleepWithJitter(current, max time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(250 * time.Millisecond)))
	time.Sleep(current + jitter)
	next := current
	if current < max {
		next = current * 2
		if next > max {
			next = max
		}
	}
	return next
}

// incrementEpoch atomically increments the fencing epoch in etcd.
// Returns the new epoch value. If etcd is unavailable, returns 0
// (the leader should still function; epoch is a safety net, not a gate).
func incrementEpoch(ctx context.Context, cli *clientv3.Client) int64 {
	epochCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Read current epoch.
	resp, err := cli.Get(epochCtx, leaderEpochKey)
	if err != nil {
		log.Printf("leader election: read epoch failed: %v", err)
		return 0
	}

	var current int64
	var modRev int64
	if len(resp.Kvs) > 0 {
		fmt.Sscanf(string(resp.Kvs[0].Value), "%d", &current)
		modRev = resp.Kvs[0].ModRevision
	}
	next := current + 1

	// CAS write: only succeed if nobody else incremented since our read.
	txnResp, err := cli.Txn(epochCtx).
		If(clientv3.Compare(clientv3.ModRevision(leaderEpochKey), "=", modRev)).
		Then(clientv3.OpPut(leaderEpochKey, fmt.Sprintf("%d", next))).
		Else(clientv3.OpGet(leaderEpochKey)).
		Commit()
	if err != nil {
		log.Printf("leader election: increment epoch failed: %v", err)
		return 0
	}
	if !txnResp.Succeeded {
		// Someone else incremented — read their value.
		if len(txnResp.Responses) > 0 {
			rangeResp := txnResp.Responses[0].GetResponseRange()
			if rangeResp != nil && len(rangeResp.Kvs) > 0 {
				fmt.Sscanf(string(rangeResp.Kvs[0].Value), "%d", &next)
			}
		}
	}
	return next
}

// readEpoch reads the current fencing epoch from etcd.
func readEpoch(ctx context.Context, cli *clientv3.Client) int64 {
	epochCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := cli.Get(epochCtx, leaderEpochKey)
	if err != nil || len(resp.Kvs) == 0 {
		return 0
	}
	var epoch int64
	fmt.Sscanf(string(resp.Kvs[0].Value), "%d", &epoch)
	return epoch
}

func sleepWithCustomJitter(current, max, jitterMax time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(jitterMax)))
	time.Sleep(current + jitter)
	next := current
	if current < max {
		next = current * 2
		if next > max {
			next = max
		}
	}
	return next
}
