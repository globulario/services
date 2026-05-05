package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestUpgradeDoesNotLoopWhenInstallSucceedsButInstalledStateCommitIsDelayed(t *testing.T) {
	origListNodeIDs := convergenceListNodeIDs
	origListResults := convergenceListResults
	origCommit := convergenceCommitWithInstall
	t.Cleanup(func() {
		convergenceListNodeIDs = origListNodeIDs
		convergenceListResults = origListResults
		convergenceCommitWithInstall = origCommit
	})

	type key struct {
		node string
		pkg  string
	}
	results := map[key]*installed_state.ConvergenceResultV1{
		{node: "n1", pkg: "workflow"}: {
			ActionID:        "n1/SERVICE/workflow/1.0.0",
			WorkflowID:      "wf-upgrade",
			Package:         "workflow",
			NodeID:          "n1",
			DesiredVersion:  "1.0.0",
			DesiredBuildID:  "build-1",
			LocalVersion:    "1.0.0",
			LocalBuildID:    "build-1",
			Outcome:         installed_state.OutcomeSuccessLocalPendingSync,
			LastAttemptAt:   time.Now().Add(-31 * time.Minute).Unix(),
			SourceComponent: "node-agent",
			Evidence:        map[string]string{"kind": "SERVICE"},
		},
	}

	convergenceListNodeIDs = func(ctx context.Context) ([]string, error) {
		return []string{"n1"}, nil
	}
	convergenceListResults = func(ctx context.Context, nodeID string) ([]*installed_state.ConvergenceResultV1, error) {
		out := make([]*installed_state.ConvergenceResultV1, 0, len(results))
		for k, v := range results {
			if k.node == nodeID {
				copyV := *v
				out = append(out, &copyV)
			}
		}
		return out, nil
	}

	var commitAttempts int
	convergenceCommitWithInstall = func(ctx context.Context, pkg *node_agentpb.InstalledPackage, r *installed_state.ConvergenceResultV1) error {
		commitAttempts++
		if commitAttempts == 1 {
			return fmt.Errorf("simulated etcd timeout")
		}
		if entry, ok := results[key{node: r.NodeID, pkg: r.Package}]; ok {
			entry.Outcome = installed_state.OutcomeSuccessCommitted
			entry.LastAttemptAt = time.Now().Unix()
		}
		return nil
	}

	srv := &server{}
	c := newConvergenceCommitter(srv)

	desired := map[string]desiredVersionInfo{
		"SERVICE/workflow": {version: "1.0.0", buildID: "build-1", buildNumber: 101},
	}

	// First pass: commit fails; result must remain pending/stale state, not deleted.
	r1 := *results[key{node: "n1", pkg: "workflow"}]
	c.processResult(context.Background(), &r1, desired)
	if got := results[key{node: "n1", pkg: "workflow"}].Outcome; got != installed_state.OutcomeSuccessLocalPendingSync {
		t.Fatalf("after failed commit, outcome=%s want %s", got, installed_state.OutcomeSuccessLocalPendingSync)
	}

	// Drift must stay suppressed while pending-sync exists.
	conv := map[string]*installed_state.ConvergenceResultV1{
		"workflow": results[key{node: "n1", pkg: "workflow"}],
	}
	if !driftSuppressed(conv, "workflow", "n1", "n1") {
		t.Fatal("pending-sync must suppress re-dispatch while commit is delayed")
	}

	// Second pass: commit succeeds and promotes to SUCCESS_COMMITTED.
	r2 := *results[key{node: "n1", pkg: "workflow"}]
	c.processResult(context.Background(), &r2, desired)
	if got := results[key{node: "n1", pkg: "workflow"}].Outcome; got != installed_state.OutcomeSuccessCommitted {
		t.Fatalf("after commit recovery, outcome=%s want %s", got, installed_state.OutcomeSuccessCommitted)
	}
	if commitAttempts < 2 {
		t.Fatalf("expected at least 2 commit attempts, got %d", commitAttempts)
	}
}

