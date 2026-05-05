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

// criticalKeyPrereqsMissing returns the first etcd key required by the package
// that is currently absent. Returns "" if all required keys are present or if
// the package kind has no prereqs. Fails open on etcd error (returns "") so a
// transient etcd failure does not permanently block deployment.
func criticalKeyPrereqsMissing(ctx context.Context, pkgName, kind string) string {
	required := make([]string, 0, len(kindCriticalKeyPrereqs[kind])+len(packageCriticalKeyPrereqs[pkgName]))
	required = append(required, kindCriticalKeyPrereqs[kind]...)
	required = append(required, packageCriticalKeyPrereqs[pkgName]...)
	if len(required) == 0 {
		return ""
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("critical-key-prereq: etcd client error for %s/%s: %v (fail-open)", kind, pkgName, err)
		return ""
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, key := range required {
		resp, err := cli.Get(tctx, key, clientv3.WithCountOnly())
		if err != nil {
			log.Printf("critical-key-prereq: etcd get %q error: %v (fail-open)", key, err)
			return ""
		}
		if resp.Count == 0 {
			return key
		}
	}
	return ""
}

// writeCriticalKeyBlock writes OutcomeBlockedCriticalKeyMissing for each node
// in nodeIDs. The action ID is deterministic so repeated calls overwrite the
// same record — LastAttemptAt is refreshed on each write, resetting the 5-minute
// re-check window tracked in driftSuppressed. Best-effort: errors are logged but
// do not abort the caller.
func writeCriticalKeyBlock(ctx context.Context, nodeIDs []string, pkgName, kind, missingKey string) {
	for _, nodeID := range nodeIDs {
		r := &installed_state.ConvergenceResultV1{
			ActionID:        criticalKeyBlockActionID(nodeID, kind, pkgName),
			WorkflowID:      "controller-preflight",
			Package:         pkgName,
			NodeID:          nodeID,
			Outcome:         installed_state.OutcomeBlockedCriticalKeyMissing,
			ReasonCode:      "missing_critical_key",
			UnblockPolicy:   "key_must_exist:" + missingKey,
			Evidence:        map[string]string{"missing_key": missingKey},
			SourceComponent: "cluster-controller",
		}
		bctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := installed_state.WriteConvergenceResult(bctx, r); err != nil {
			log.Printf("critical-key-prereq: write block for %s/%s on %s: %v", kind, pkgName, nodeID, err)
		}
		cancel()
	}
}

func criticalKeyBlockActionID(nodeID, kind, pkgName string) string {
	return fmt.Sprintf("controller/%s/%s/%s/critical_key_block", nodeID, kind, pkgName)
}
