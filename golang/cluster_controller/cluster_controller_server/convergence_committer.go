package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// convergenceCommitter is a leader-only background goroutine that reads
// ConvergenceResultV1 records written by node-agents and commits authoritative
// InstalledPackage state to etcd.
//
// This closes the loop-causing gap:
//   - node-agent runInstallPackage: skips reinstall (installSkipAllowed) but
//     etcd build_number may be stale → drift reconciler keeps re-dispatching.
//   - node-agent emits ConvergenceResultV1 → committer reads it and writes a
//     complete InstalledPackage (version + build_id + build_number from desired).
//
// Only the controller leader runs this committer. Non-leaders skip each tick.
type convergenceCommitter struct {
	srv      *server
	interval time.Duration
	commitSem chan struct{}
}

var (
	convergenceListNodeIDs       = installed_state.ListNodeIDs
	convergenceListResults       = installed_state.ListConvergenceResults
	convergenceWriteResult       = installed_state.WriteConvergenceResult
	convergenceCommitWithInstall = installed_state.CommitConvergenceWithInstall
	convergenceDeleteResult      = installed_state.DeleteConvergenceResult
)

const (
	defaultMaxParallelPackageCommits = 2
)

func newConvergenceCommitter(srv *server) *convergenceCommitter {
	return &convergenceCommitter{
		srv:       srv,
		interval:  15 * time.Second,
		commitSem: make(chan struct{}, defaultMaxParallelPackageCommits),
	}
}

// Start launches the committer as a background goroutine under safeGo.
func (c *convergenceCommitter) Start(ctx context.Context) {
	safeGo("convergence-committer", func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !c.srv.isLeader() {
					continue
				}
				c.runOnce(ctx)
			}
		}
	})
}

// Pending-sync thresholds (PR-5 contract defaults).
// Keep as constants for now; make configurable in a later PR.
const (
	pendingSyncWarnAfter  = 5 * time.Minute
	pendingSyncStaleAfter = 30 * time.Minute
)

func (c *convergenceCommitter) runOnce(ctx context.Context) {
	nodeIDs, err := convergenceListNodeIDs(ctx)
	if err != nil {
		log.Printf("convergence-committer: list node IDs: %v", err)
		return
	}

	desired := c.srv.collectDesiredVersions(ctx)
	warnThreshold := time.Now().Add(-pendingSyncWarnAfter).Unix()
	staleThreshold := time.Now().Add(-pendingSyncStaleAfter).Unix()

	for _, nodeID := range nodeIDs {
		results, err := convergenceListResults(ctx, nodeID)
		if err != nil {
			log.Printf("convergence-committer: list results node=%s: %v", nodeID, err)
			continue
		}
		for _, r := range results {
			if r.Outcome == installed_state.OutcomeSuccessLocalPendingSync && r.LastAttemptAt > 0 {
				switch {
				case r.LastAttemptAt < staleThreshold:
					log.Printf("convergence-committer: STALE pending-sync node=%s pkg=%s since %s — escalating to STALE_INSTALLED_STATE",
						r.NodeID, r.Package, time.Unix(r.LastAttemptAt, 0).Format(time.RFC3339))
					stale := *r
					stale.Outcome = installed_state.OutcomeStaleInstalledState
					stale.SourceComponent = "cluster-controller"
					stale.ReasonCode = "PENDING_SYNC_STALE"
					if err := convergenceWriteResult(ctx, &stale); err != nil {
						log.Printf("convergence-committer: promote stale pending-sync node=%s pkg=%s: %v", r.NodeID, r.Package, err)
					}
				case r.LastAttemptAt < warnThreshold:
					log.Printf("convergence-committer: WARNING pending-sync node=%s pkg=%s since %s — controller may have lost etcd connectivity",
						r.NodeID, r.Package, time.Unix(r.LastAttemptAt, 0).Format(time.RFC3339))
				}
			}
			c.processResult(ctx, r, desired)
		}
	}
}

func (c *convergenceCommitter) processResult(ctx context.Context, r *installed_state.ConvergenceResultV1, desired map[string]desiredVersionInfo) {
	switch r.Outcome {
	case installed_state.OutcomeSuccessLocalPendingSync:
		c.commitSem <- struct{}{}
		defer func() { <-c.commitSem }()

		kind := strings.ToUpper(strings.TrimSpace(r.Evidence["kind"]))
		if kind == "" {
			kind = "SERVICE"
		}

		// Read existing record to preserve InstalledUnix, Checksum, Platform, etc.
		pkg, _ := installed_state.GetInstalledPackage(ctx, r.NodeID, kind, r.Package)
		if pkg == nil {
			pkg = &node_agentpb.InstalledPackage{
				NodeId: r.NodeID,
				Name:   r.Package,
				Kind:   kind,
			}
		}

		// Apply convergence truth from the result.
		pkg.Version = r.LocalVersion
		if r.LocalBuildID != "" {
			pkg.BuildId = r.LocalBuildID
		}
		pkg.Status = "installed"
		pkg.UpdatedUnix = time.Now().Unix()

		// Stamp build_number from current desired state so the drift reconciler
		// sees EqualFull() match and stops re-dispatching.
		desiredKey := kind + "/" + canonicalServiceName(r.Package)
		if dv, ok := desired[desiredKey]; ok && dv.buildNumber > 0 {
			pkg.BuildNumber = dv.buildNumber
		}

		// Atomic Txn: write installed-package + promote convergence result to
		// OutcomeSuccessCommitted + delete convergence action key — all in one
		// etcd operation. If the controller crashes between any of the previous
		// separate writes the next leader will retry from OutcomeSuccessLocalPendingSync.
		if err := convergenceCommitWithInstall(ctx, pkg, r); err != nil {
			log.Printf("convergence-committer: atomic commit node=%s %s/%s: %v",
				r.NodeID, kind, r.Package, err)
			return
		}
		log.Printf("convergence-committer: committed node=%s %s/%s@%s build_number=%d",
			r.NodeID, kind, r.Package, pkg.Version, pkg.BuildNumber)

	case installed_state.OutcomeSuccessCommitted:
		// Committed by a previous controller pass; action key may linger if the
		// delete lost a race. Clean it up now.
		if err := convergenceDeleteResult(ctx, r.ActionID); err != nil {
			log.Printf("convergence-committer: delete committed result %s: %v", r.ActionID, err)
		}

	case installed_state.OutcomeFailedTransient,
		installed_state.OutcomeFailedPermanent,
		installed_state.OutcomeDegradedRetrying:
		// Leave non-success results in place for diagnosis / operator tooling.
		log.Printf("convergence-committer: node=%s %s/%s outcome=%s reason=%s",
			r.NodeID, r.Evidence["kind"], r.Package, r.Outcome, r.ReasonCode)

	default:
		// BLOCKED_* and STALE_INSTALLED_STATE are informational — leave in place.
	}
}
