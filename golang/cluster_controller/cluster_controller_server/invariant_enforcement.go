package main

// invariant_enforcement.go — Cluster-wide invariant enforcement.
//
// These checks run in the reconcile loop WITHOUT depending on MinIO or the
// workflow service. They are the last line of defense — if everything else
// is broken, these still run because they only need etcd and local state.
//
// The workflow definition (cluster.invariant.enforcement.yaml) wraps these
// same checks for auditability when the workflow service is healthy.
//
// Invariants enforced:
//   1. Infrastructure quorum: etcd (all nodes), ScyllaDB (≥3), MinIO (≥3)
//   2. Founding profiles: first 3 nodes MUST have core+control-plane+storage
//   3. Workflow completeness: all required definitions exist in MinIO
//
// These run under srv.lock() during the reconcile snapshot phase.

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// coreWorkflows are the workflow definitions that MUST live in etcd. These
// are the workflows the cluster needs to bootstrap, reconcile, deploy, and
// heal itself. Without them, nothing works.
// Service-owned workflows (compute.*, remediate.*) stay in MinIO.
var coreWorkflows = []string{
	"cluster.reconcile",
	"cluster.invariant.enforcement",
	"node.bootstrap",
	"node.join",
	"node.repair",
	"release.apply.package",
	"release.apply.infrastructure",
	"release.apply.controller",
	"release.remove.package",
}

// enforceAllInvariantsLocked runs all cluster invariants during the reconcile
// snapshot phase. MUST be called under srv.lock(). Returns true if state was modified.
func (srv *server) enforceAllInvariantsLocked() bool {
	modified := false

	// 1. Infrastructure quorum — auto-promote nodes if needed.
	if srv.enforceStorageQuorumLocked() {
		modified = true
	}

	// 2. Workflow completeness — throttled to avoid hammering MinIO.
	//    We just flag it here; the actual repair happens after unlock.
	if time.Since(lastWorkflowCheck) >= workflowCheckInterval {
		lastWorkflowCheck = time.Now()
		srv.workflowRepairNeeded = srv.checkWorkflowCompleteness()
	}

	return modified
}

// checkWorkflowCompleteness checks if all core workflow definitions exist
// in etcd. Returns the list of missing workflow names. Does NOT require
// MinIO or the workflow service — reads etcd directly.
func (srv *server) checkWorkflowCompleteness() []string {
	if v1alpha1.EtcdFetcher == nil {
		return nil
	}
	var missing []string
	for _, name := range coreWorkflows {
		if b, err := v1alpha1.EtcdFetcher(name); err != nil || len(b) == 0 {
			missing = append(missing, name)
		}
	}
	return missing
}

// repairMissingWorkflows re-seeds missing core workflow definitions from the
// local filesystem to etcd. Called AFTER srv.unlock() since it does I/O.
// This self-heals etcd data loss for workflow definitions.
func (srv *server) repairMissingWorkflows(ctx context.Context, missing []string) {
	if len(missing) == 0 {
		return
	}

	log.Printf("invariant: %d core workflow definitions missing from etcd, repairing: %v", len(missing), missing)

	defs := loadCoreWorkflowsFromDisk()
	repair := make(map[string][]byte)
	for _, name := range missing {
		if data, ok := defs[name]; ok {
			repair[name] = data
		} else {
			log.Printf("invariant: workflow %s not found on local disk, cannot repair", name)
		}
	}

	if len(repair) > 0 {
		if err := v1alpha1.SeedCoreWorkflows(repair); err != nil {
			log.Printf("invariant: seed repair failed: %v", err)
		} else {
			srv.emitClusterEvent("controller.workflows_repaired", map[string]interface{}{
				"repaired_count": len(repair),
				"missing_count":  len(missing),
				"workflows":      missing,
			})
		}
	}
}

// seedCoreWorkflowsToEtcd loads core workflow YAMLs from disk and writes them
// to etcd. Called once when the controller gains leadership.
func (srv *server) seedCoreWorkflowsToEtcd() {
	defs := loadCoreWorkflowsFromDisk()
	if len(defs) == 0 {
		log.Printf("workflow-seed: no core workflow definitions found on disk")
		return
	}
	if err := v1alpha1.SeedCoreWorkflows(defs); err != nil {
		log.Printf("workflow-seed: failed to seed core workflows: %v", err)
	}
}

// workflowDefinitionsDir is where workflow YAML files are installed on disk.
// The installer places them here; the controller reads them at startup.
var workflowDefinitionsDir = "/var/lib/globular/workflows"

// loadCoreWorkflowsFromDisk reads core workflow YAML files from the local
// filesystem. Returns a map of name → YAML content.
func loadCoreWorkflowsFromDisk() map[string][]byte {
	defs := make(map[string][]byte)
	for _, name := range coreWorkflows {
		path := filepath.Join(workflowDefinitionsDir, name+".yaml")
		data, err := readFileIfExists(path)
		if err != nil || len(data) == 0 {
			continue
		}
		defs[name] = data
	}
	return defs
}

// lastWorkflowCheck tracks when we last checked workflow completeness
// to avoid hitting MinIO every reconcile cycle.
var lastWorkflowCheck time.Time
const workflowCheckInterval = 5 * time.Minute

// readFileIfExists reads a file or returns empty if it doesn't exist.
func readFileIfExists(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}
