package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// Drift episode identity.
//
// A drift item is "unresolved" while it keeps appearing in consecutive
// scan_drift outputs. workflow.drift_unresolved holds one row per
// (cluster_id, drift_type, entity_ref); RecordDriftObservation PRESERVES
// first_observed_at while the row exists and mints it only when the row is
// absent, and ClearDriftObservation DELETES the row once the item stops being
// observed. first_observed_at is therefore the persisted identity of one
// unresolved-row INCARNATION:
//
//   - stable across repeated reconcile ticks (the row is not rewritten),
//   - stable across controller and Workflow Service restarts (it lives in
//     ScyllaDB, not process memory),
//   - and different for a later episode, because resolution deleted the row
//     and the next observation minted a new value.
//
// That is exactly the distinction the correlation identity needs, and it is
// minted by the drift-history authority when the episode begins rather than by
// each dispatch attempt.

// errNoDriftEpisode reports that the episode identity could not be proven.
// Callers MUST fail closed on it: no restart run may be created from a guessed
// identity, and the drift must stay unresolved so a later tick can retry once
// the authority has the row.
type errNoDriftEpisode struct {
	driftType string
	entityRef string
	reason    string
}

func (e *errNoDriftEpisode) Error() string {
	return fmt.Sprintf("drift episode identity unavailable for %s/%s: %s",
		e.driftType, e.entityRef, e.reason)
}

// episodeIDFromDriftRows extracts the episode identity for one drift item from
// the drift-history authority's rows.
//
// Pure function over persisted state: given the same row it returns the same
// id in any process, which is what makes the correlation identity survive a
// controller restart, a Workflow Service restart, and a lost dispatch response
// without any process-local memory.
func episodeIDFromDriftRows(items []*workflowpb.DriftUnresolved, driftType, entityRef string) (string, error) {
	driftType = strings.TrimSpace(driftType)
	entityRef = strings.TrimSpace(entityRef)
	if driftType == "" || entityRef == "" {
		return "", &errNoDriftEpisode{driftType, entityRef, "drift_type and entity_ref are required"}
	}
	for _, it := range items {
		if it == nil {
			continue
		}
		if strings.TrimSpace(it.GetDriftType()) != driftType ||
			strings.TrimSpace(it.GetEntityRef()) != entityRef {
			continue
		}
		ts := it.GetFirstObservedAt()
		if ts == nil {
			return "", &errNoDriftEpisode{driftType, entityRef,
				"unresolved row carries no first_observed_at"}
		}
		at := ts.AsTime()
		if at.IsZero() {
			return "", &errNoDriftEpisode{driftType, entityRef,
				"unresolved row carries a zero first_observed_at"}
		}
		// UTC + RFC3339Nano: a stable, readable rendering of the authority's
		// own value. Never re-stamped here — this formats what was persisted.
		return at.UTC().Format(time.RFC3339Nano), nil
	}
	return "", &errNoDriftEpisode{driftType, entityRef,
		"no unresolved drift row (episode not yet recorded, or already cleared)"}
}

// driftEpisodeID asks the drift-history authority for the current episode
// identity of one drift item.
// Every absent authority object below is a MISSING EPISODE, never a panic and
// never a synthesized identity. The distinction matters because this runs on
// the reconcile path: a controller that is still assembling itself, or that
// lost its workflow client, must decline to remediate rather than crash the
// reconcile loop or invent an episode to key a restart on.
//
// Mirrors security.cluster_init_transition_is_fail_closed: an incompletely
// initialized controller resolves to "cannot proceed", not to a default that
// happens to let the action through.
func (srv *server) driftEpisodeID(ctx context.Context, driftType, entityRef string) (string, error) {
	if srv == nil {
		return "", &errNoDriftEpisode{driftType, entityRef, "controller is not initialized"}
	}
	if srv.cfg == nil {
		return "", &errNoDriftEpisode{driftType, entityRef, "controller configuration is unavailable"}
	}
	clusterID := strings.TrimSpace(srv.cfg.ClusterDomain)
	if clusterID == "" {
		return "", &errNoDriftEpisode{driftType, entityRef, "cluster domain is not configured"}
	}
	wc := srv.getWorkflowClient()
	if wc == nil {
		return "", &errNoDriftEpisode{driftType, entityRef, "workflow client unavailable"}
	}
	resp, err := wc.ListDriftUnresolved(ctx, &workflowpb.ListDriftUnresolvedRequest{
		ClusterId: clusterID,
		MinCycles: 1,
	})
	if err != nil {
		return "", &errNoDriftEpisode{driftType, entityRef,
			fmt.Sprintf("drift-history authority unreachable: %v", err)}
	}
	if resp == nil {
		return "", &errNoDriftEpisode{driftType, entityRef, "drift-history authority returned no response"}
	}
	return episodeIDFromDriftRows(resp.GetItems(), driftType, entityRef)
}
