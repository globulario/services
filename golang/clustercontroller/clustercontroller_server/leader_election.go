package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const leaderElectionPrefix = "/globular/clustercontroller/leader"
const publishAttemptTimeout = 2 * time.Second

var seedRandOnce sync.Once

func seedRand() {
	seedRandOnce.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
}

func startLeaderElection(ctx context.Context, cli *clientv3.Client, srv *server, addr string) {
	seedRand()
	safeGo("leader-election", func() {
		backoff := 250 * time.Millisecond
		maxBackoff := 5 * time.Second
		resetBackoff := func() { backoff = 250 * time.Millisecond }
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			ttl := 15
			if env := strings.TrimSpace(os.Getenv("CLUSTER_CONTROLLER_LEASE_TTL_SECONDS")); env != "" {
				if v, err := strconv.Atoi(env); err == nil && v > 0 {
					ttl = v
				}
			}
			sess, err := concurrency.NewSession(cli, concurrency.WithTTL(ttl))
			if err != nil {
				log.Printf("leader election: create session failed: %v", err)
				backoff = sleepWithJitter(backoff, maxBackoff)
				continue
			}
			election := concurrency.NewElection(sess, leaderElectionPrefix)
			host, _ := os.Hostname()
			candidateID := fmt.Sprintf("%s:%d:%s", host, os.Getpid(), uuid.NewString())
			if err := election.Campaign(ctx, candidateID); err != nil {
				log.Printf("leader election: campaign failed: %v", err)
				_ = sess.Close()
				backoff = sleepWithJitter(backoff, maxBackoff)
				continue
			}
			// Reset backoff after successful campaign
			resetBackoff()
			// Gained leadership
			srv.setLeader(true, candidateID, addr)
			if err := publishLeaderAddr(ctx, cli, sess.Lease(), addr); err != nil {
				log.Printf("leader election: publish addr failed: %v", err)
			} else {
				log.Printf("leader election: became leader id=%s addr=%s", candidateID, addr)
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
