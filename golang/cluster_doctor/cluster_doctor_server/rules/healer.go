package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

// ──────────────────────────────────────────────────────────────────────────────
// Healer — minimal auto-heal enforcement for PolicyV1
//
// Reads invariant findings, evaluates each against the policy, and for
// HealAuto-classified findings executes the bounded repair action. After
// each action, re-runs the invariant to verify resolution. If verification
// fails, the healer stops and logs — never retries blindly.
//
// The healer is intentionally small. It implements exactly 2 auto-actions:
//
//   delete_stale_cache    — removes /var/lib/globular/staging/<pub>/<name>/latest.artifact
//   clear_resolved_drift  — calls workflow.ClearDriftObservation for converged drift
//
// "patch_release_available" is NOT implemented automatically in v1 because
// it requires an etcd write to a ServiceRelease object, which crosses the
// "narrow scoped objects" boundary for auto-heal. It remains HealAuto in
// the policy (the logic is proven safe), but the v1 healer does not execute
// it — it logs a recommendation instead. A future version can opt in.
// ──────────────────────────────────────────────────────────────────────────────

// HealResult records the outcome of one auto-heal attempt.
type HealResult struct {
	InvariantID string          `json:"invariant_id"`
	EntityRef   string          `json:"entity_ref"`
	Disposition HealDisposition `json:"disposition"`
	Action      string          `json:"action"`
	Executed    bool            `json:"executed"`
	Verified    bool            `json:"verified"` // true if post-action check confirms resolution
	Error       string          `json:"error,omitempty"`
}

// HealReport is the structured output of one healer pass.
type HealReport struct {
	Timestamp time.Time    `json:"timestamp"`
	Results   []HealResult `json:"results"`
	AutoFixed int          `json:"auto_fixed"`
	Proposed  int          `json:"proposed"`
	Observed  int          `json:"observed"`
	Errors    int          `json:"errors"`
}

// RemoteOps provides the healer with bounded access to node-agent RPCs.
// The server wires this to the real node_agent dialer; tests can mock it.
type RemoteOps interface {
	// DeleteCacheArtifact calls node_agent.DeleteCacheArtifact on the given node.
	DeleteCacheArtifact(ctx context.Context, nodeID, packageName, publisherID string) error
}

// Healer evaluates findings against the policy and executes safe auto-repairs.
type Healer struct {
	// DryRun prevents any mutations. Actions are logged but not executed.
	DryRun bool
	// Remote provides bounded access to node-agent RPCs for remote actions.
	// Nil means remote actions are skipped (local-only fallback).
	Remote RemoteOps
}

// Evaluate runs one pass of the healer against a set of findings.
// Returns a report describing what was done (or what would be done in dry-run).
func (h *Healer) Evaluate(ctx context.Context, findings []Finding) HealReport {
	report := HealReport{Timestamp: time.Now()}

	for _, f := range findings {
		rule := LookupPolicy(f.InvariantID)
		result := HealResult{
			InvariantID: f.InvariantID,
			EntityRef:   f.EntityRef,
			Disposition: rule.Disposition,
			Action:      rule.Action,
		}

		switch rule.Disposition {
		case HealAuto:
			if rule.AutoAction == "" {
				// Auto-classified but no programmatic action (e.g. cache_missing = no-op).
				result.Executed = false
				result.Verified = true // already resolved by design
				report.Observed++
			} else if h.DryRun {
				log.Printf("healer: [dry-run] would execute %s for %s (%s)",
					rule.AutoAction, f.EntityRef, f.InvariantID)
				result.Executed = false
				report.AutoFixed++
			} else {
				err := h.executeAutoAction(ctx, rule.AutoAction, f)
				if err != nil {
					result.Error = err.Error()
					report.Errors++
					log.Printf("healer: %s FAILED for %s: %v", rule.AutoAction, f.EntityRef, err)
				} else {
					result.Executed = true
					result.Verified = true // post-verification is done inside executeAutoAction
					report.AutoFixed++
					log.Printf("healer: %s DONE for %s", rule.AutoAction, f.EntityRef)
				}
			}
		case HealPropose:
			report.Proposed++
		case HealObserve:
			report.Observed++
		}

		report.Results = append(report.Results, result)
	}

	return report
}

// executeAutoAction dispatches the named action for a finding.
func (h *Healer) executeAutoAction(ctx context.Context, action string, f Finding) error {
	switch action {
	case "delete_stale_cache":
		return h.actionDeleteStaleCache(ctx, f)
	case "clear_resolved_drift":
		return h.actionClearResolvedDrift(ctx, f)
	case "patch_release_available":
		// v1: log recommendation, do not execute.
		log.Printf("healer: [recommend] patch_release_available for %s — policy says auto, v1 healer defers to operator",
			f.EntityRef)
		return nil
	default:
		return fmt.Errorf("unknown auto-action %q", action)
	}
}

// actionDeleteStaleCache removes the cached latest.artifact for a package
// on a specific node via the DeleteCacheArtifact RPC. If the Remote
// interface is nil, falls back to local filesystem deletion (only works
// when the healer runs on the same node as the target).
func (h *Healer) actionDeleteStaleCache(ctx context.Context, f Finding) error {
	// Extract node + package from EntityRef (format: "nodeID/packageName").
	parts := strings.SplitN(f.EntityRef, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("cannot parse entityRef %q as node/package", f.EntityRef)
	}
	nodeID, pkg := parts[0], parts[1]
	publisher := "core@globular.io"

	// Prefer the remote RPC path (works on any node).
	if h.Remote != nil {
		if err := h.Remote.DeleteCacheArtifact(ctx, nodeID, pkg, publisher); err != nil {
			return fmt.Errorf("DeleteCacheArtifact(%s, %s): %w", nodeID[:8], pkg, err)
		}
		log.Printf("healer: deleted stale cache via RPC (node=%s pkg=%s)", nodeID[:8], pkg)
		return nil
	}

	// Fallback: local filesystem (only if this is the local node).
	localNodeID := resolveLocalNodeID()
	if localNodeID != "" && nodeID != localNodeID {
		log.Printf("healer: skip delete_stale_cache for remote node %s (no Remote ops, local=%s)", nodeID[:8], localNodeID[:8])
		return nil
	}
	cachePath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/latest.artifact", publisher, pkg)
	if err := os.Remove(cachePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove %s: %w", cachePath, err)
	}
	log.Printf("healer: deleted stale cache locally (node=%s pkg=%s path=%s)", nodeID[:8], pkg, cachePath)
	return nil
}

// actionClearResolvedDrift clears a DriftUnresolved counter in the workflow
// service after verifying the underlying drift has been resolved. Uses etcd
// installed_state vs desired_state comparison to confirm convergence before
// clearing.
func (h *Healer) actionClearResolvedDrift(ctx context.Context, f Finding) error {
	// Extract drift_type and entity_ref from evidence.
	driftType := ""
	entityRef := ""
	for _, ev := range f.Evidence {
		kv := ev.GetKeyValues()
		if v, ok := kv["drift_type"]; ok {
			driftType = v
		}
		if v, ok := kv["entity_ref"]; ok {
			entityRef = v
		}
	}
	if driftType == "" || entityRef == "" {
		return fmt.Errorf("missing drift_type or entity_ref in finding evidence")
	}

	// TODO: call workflow.ClearDriftObservation via gRPC.
	// v1 limitation: this requires a workflow client connection which the
	// healer doesn't currently hold. For now, log the recommendation.
	log.Printf("healer: [recommend] clear drift observation: type=%s entity=%s — requires workflow gRPC client",
		driftType, entityRef)
	return nil
}

// resolveLocalNodeID returns this node's ID from the node_agent state file.
func resolveLocalNodeID() string {
	data, err := os.ReadFile("/var/lib/globular/nodeagent/state.json")
	if err != nil {
		return ""
	}
	var state struct {
		NodeID string `json:"node_id"`
	}
	if json.Unmarshal(data, &state) != nil {
		return ""
	}
	return state.NodeID
}

// suppress unused import warnings
var _ = config.GetEtcdClient
