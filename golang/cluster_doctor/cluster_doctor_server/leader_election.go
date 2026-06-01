// leader_election.go implements etcd-based leader election for the
// cluster-doctor. Only the elected leader produces fresh findings;
// followers serve cached/stale data with explicit source disclosure.
//
// Uses the same concurrency.Election pattern as the cluster-controller.
// See docs/architecture/HA-control-plane-design.md §Class B.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/google/uuid"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const doctorLeaderPrefix = "/globular/cluster_doctor/leader"

// startDoctorLeaderElection runs the leader election loop in a background
// goroutine. When leadership is gained, srv.isAuthoritative is set to true.
// When lost, it's cleared. The election prefix is separate from the
// controller's prefix — doctor and controller elect independently.
func startDoctorLeaderElection(ctx context.Context, srv *ClusterDoctorServer) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		logger.Warn("doctor leader election: etcd unavailable, running as sole authority", "err", err)
		srv.isAuthoritative.Store(true)
		return
	}

	go func() {
		backoff := 250 * time.Millisecond
		maxBackoff := 5 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ttl := 15 // seconds
			sess, err := concurrency.NewSession(cli, concurrency.WithTTL(ttl))
			if err != nil {
				logger.Warn("doctor leader election: session failed", "err", err)
				time.Sleep(backoff)
				if backoff < maxBackoff {
					backoff *= 2
				}
				continue
			}

			election := concurrency.NewElection(sess, doctorLeaderPrefix)
			host, _ := os.Hostname()
			candidateID := fmt.Sprintf("doctor:%s:%d:%s", host, os.Getpid(), uuid.NewString())

			logger.Info("doctor leader election: campaigning", "candidate", candidateID)
			if err := election.Campaign(ctx, candidateID); err != nil {
				logger.Warn("doctor leader election: campaign failed", "err", err)
				_ = sess.Close()
				time.Sleep(backoff)
				if backoff < maxBackoff {
					backoff *= 2
				}
				continue
			}

			// Won leadership.
			backoff = 250 * time.Millisecond
			srv.isAuthoritative.Store(true)
			logger.Info("doctor leader election: became leader", "candidate", candidateID)

			// Hold leadership until session expires or context is cancelled.
			select {
			case <-ctx.Done():
				srv.isAuthoritative.Store(false)
				resignCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = election.Resign(resignCtx)
				cancel()
				_ = sess.Close()
				return
			case <-sess.Done():
				// Lost session (lease expired, etcd unavailable).
				srv.isAuthoritative.Store(false)
				logger.Warn("doctor leader election: lost session, demoted to follower")
			}

			_ = sess.Close()
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
			}
		}
	}()
}

// isAuthoritative is stored on ClusterDoctorServer. It's an atomic.Bool
// that the leader election goroutine manages. RPC handlers check this
// to decide whether to produce fresh findings or serve cached.
//
// Field is declared on the server struct in server.go.

// authoritySource returns "leader" if this instance is the elected authority,
// or "follower" otherwise. Used in freshness headers.
func (s *ClusterDoctorServer) authoritySource() string {
	if s.isAuthoritative.Load() {
		return "leader"
	}
	return "follower"
}
