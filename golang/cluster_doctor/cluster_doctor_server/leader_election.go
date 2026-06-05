// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.leader_election
// @awareness file_role=etcd_leader_election_gates_fresh_finding_production_and_remediation
// @awareness implements=globular.platform:intent.remediation.must_go_through_workflow
// @awareness implements=globular.platform:intent.doctor.findings_are_operator_language
// @awareness risk=high
//
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
		// Dedup state for repeated session-failure errors — enforces
		// meta.diagnostic_output_must_be_bounded. A persistent etcd
		// outage would otherwise produce one "session failed" line
		// per backoff tick (up to 720/hr at the 5s max-backoff).
		const errLogDedupWindow = 5 * time.Minute
		var (
			lastSessErr     string
			lastSessErrAt   time.Time
			sessErrSuppress int
		)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ttl := 15 // seconds
			sess, err := concurrency.NewSession(cli, concurrency.WithTTL(ttl))
			if err != nil {
				msg := err.Error()
				if msg == lastSessErr && time.Since(lastSessErrAt) < errLogDedupWindow {
					sessErrSuppress++
				} else {
					if sessErrSuppress > 0 {
						logger.Warn("doctor leader election: session failed",
							"err", lastSessErr, "repeated", sessErrSuppress)
					} else {
						logger.Warn("doctor leader election: session failed", "err", err)
					}
					lastSessErr = msg
					lastSessErrAt = time.Now()
					sessErrSuppress = 0
				}
				time.Sleep(backoff)
				if backoff < maxBackoff {
					backoff *= 2
				}
				continue
			}
			// Clear dedup state on successful session creation.
			if lastSessErr != "" {
				logger.Info("doctor leader election: session recovered",
					"prior_err", lastSessErr, "suppressed", sessErrSuppress)
				lastSessErr = ""
				sessErrSuppress = 0
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
