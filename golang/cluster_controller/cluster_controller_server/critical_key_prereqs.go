package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// kindCriticalKeyPrereqs maps package kind to the etcd keys that must exist
// before dispatch. Infrastructure packages are excluded — they create their own
// config rather than consuming it. Commands require no prereqs.
var kindCriticalKeyPrereqs = map[string][]string{
	"SERVICE":  {"/globular/system/config"},
	"WORKLOAD": {"/globular/system/config"},
	// INFRASTRUCTURE and COMMAND: no prereqs
}

// packageCriticalKeyPrereqs maps installed-state-name to ADDITIONAL required
// etcd keys beyond the kind-level prereqs. A package listed here must wait for
// all keys — kind prereqs AND package prereqs — before dispatch proceeds.
var packageCriticalKeyPrereqs = map[string][]string{
	"keepalived": {"/globular/ingress/v1/spec"},
	"envoy":      {"/globular/ingress/v1/spec"},
}

var (
	criticalKeyGetEtcdClient = config.GetEtcdClient
	criticalKeyWriteResult   = installed_state.WriteConvergenceResult
)

// criticalKeyPrereqStatus evaluates required critical keys for a package dispatch.
// Returns:
//   - missingKey: first missing key (non-empty when key absent)
//   - checkErr: query execution error (etcd/TLS/path); dispatch must be blocked
//
// If kind/pkg has no prereqs, both return empty.
func criticalKeyPrereqStatus(ctx context.Context, pkgName, kind string) (missingKey string, checkErr error) {
	required := make([]string, 0, len(kindCriticalKeyPrereqs[kind])+len(packageCriticalKeyPrereqs[pkgName]))
	required = append(required, kindCriticalKeyPrereqs[kind]...)
	required = append(required, packageCriticalKeyPrereqs[pkgName]...)
	if len(required) == 0 {
		return "", nil
	}
	cli, err := criticalKeyGetEtcdClient()
	if err != nil {
		return "", fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, key := range required {
		resp, err := cli.Get(tctx, key, clientv3.WithCountOnly())
		if err != nil {
			return "", fmt.Errorf("get %s: %w", key, err)
		}
		if resp.Count == 0 {
			return key, nil
		}
	}
	return "", nil
}

// writeCriticalKeyBlock writes OutcomeBlockedCriticalKeyMissing for each node
// in nodeIDs. The action ID is deterministic so repeated calls overwrite the
// same record — LastAttemptAt is refreshed on each write, resetting the 5-minute
// re-check window tracked in driftSuppressed. Best-effort: errors are logged but
// do not abort the caller.
func writeCriticalKeyBlock(ctx context.Context, nodeIDs []string, pkgName, kind, missingKey string, checkErr error) {
	for _, nodeID := range nodeIDs {
		reasonCode := "missing_critical_key"
		unblockPolicy := "key_must_exist:" + missingKey
		evidence := map[string]string{"missing_key": missingKey}
		if checkErr != nil {
			reasonCode = "critical_key_check_error"
			unblockPolicy = "check_error_retry_after_backoff"
			evidence = map[string]string{
				"check_error": checkErr.Error(),
			}
		}
		r := &installed_state.ConvergenceResultV1{
			ActionID:        criticalKeyBlockActionID(nodeID, kind, pkgName),
			WorkflowID:      "controller-preflight",
			Package:         pkgName,
			NodeID:          nodeID,
			Outcome:         installed_state.OutcomeBlockedCriticalKeyMissing,
			ReasonCode:      reasonCode,
			UnblockPolicy:   unblockPolicy,
			Evidence:        evidence,
			SourceComponent: "cluster-controller",
		}
		bctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := criticalKeyWriteResult(bctx, r); err != nil {
			log.Printf("critical-key-prereq: write block for %s/%s on %s: %v", kind, pkgName, nodeID, err)
		}
		cancel()
	}
}

func criticalKeyBlockActionID(nodeID, kind, pkgName string) string {
	return fmt.Sprintf("controller/%s/%s/%s/critical_key_block", nodeID, kind, pkgName)
}

// clearRuntimeDepBlock deletes stale dep-block records for nodes whose
// RuntimeLocalDependencies are now satisfied. Both the action key and the
// latest-outcome key are deleted so the reconciler re-dispatches the install.
// Best-effort: errors are logged but do not abort the caller.
func clearRuntimeDepBlock(ctx context.Context, nodeIDs []string, pkgName, kind string) {
	for _, nodeID := range nodeIDs {
		actionID := fmt.Sprintf("controller/%s/%s/%s/runtime_dep_block", nodeID, kind, pkgName)
		// Delete the action record (written by writeRuntimeDepBlock).
		bctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := installed_state.DeleteConvergenceResult(bctx, actionID); err != nil {
			log.Printf("runtime-dep-block: clear action for %s/%s on %s: %v", kind, pkgName, nodeID, err)
		}
		cancel()
		// Delete the latest-outcome record that blocks hasUnservedNodes.
		cli, cerr := criticalKeyGetEtcdClient()
		if cerr != nil {
			log.Printf("runtime-dep-block: etcd client for latest clear %s/%s on %s: %v", kind, pkgName, nodeID, cerr)
			continue
		}
		lctx, lcancel := context.WithTimeout(ctx, 5*time.Second)
		latestKey := installed_state.ConvergenceLatestKey(nodeID, pkgName)
		if _, err := cli.Delete(lctx, latestKey); err != nil {
			log.Printf("runtime-dep-block: clear latest for %s/%s on %s: %v", kind, pkgName, nodeID, err)
		}
		lcancel()
	}
}

// writeRuntimeDepBlock writes OutcomeBlockedMissingNativeDep for each node in
// nodeIDs. Called when reconcileResolved skips a node because its runtime local
// dependencies (e.g. minio for sidekick) are not yet active. The record is
// picked up by convergenceBlockedNodes so hasUnservedNodes skips the node,
// breaking the AVAILABLE → PENDING → no-op spin loop.
//
// The action ID is deterministic — repeated calls overwrite the same record so
// no stale accumulation occurs. The record is superseded by any successful
// convergence result written by the node-agent on successful install.
func writeRuntimeDepBlock(ctx context.Context, nodeIDs []string, pkgName, kind string, missing []string) {
	missingStr := fmt.Sprintf("%v", missing)
	for _, nodeID := range nodeIDs {
		r := &installed_state.ConvergenceResultV1{
			ActionID:        fmt.Sprintf("controller/%s/%s/%s/runtime_dep_block", nodeID, kind, pkgName),
			WorkflowID:      "controller-preflight",
			Package:         pkgName,
			NodeID:          nodeID,
			Outcome:         installed_state.OutcomeBlockedMissingNativeDep,
			ReasonCode:      "runtime_deps_not_ready",
			UnblockPolicy:   "deps_must_be_active:" + missingStr,
			Evidence:        map[string]string{"missing_deps": missingStr},
			SourceComponent: "cluster-controller",
		}
		bctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := criticalKeyWriteResult(bctx, r); err != nil {
			log.Printf("runtime-dep-block: write block for %s/%s on %s: %v", kind, pkgName, nodeID, err)
		}
		cancel()
	}
}
