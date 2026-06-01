// @awareness namespace=globular.platform
// @awareness component=platform_controller.reconciler
// @awareness file_role=package_kind_mismatch_detection_in_etcd
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
)

const kindMismatchEtcdPrefix = "/globular/controller/kind_mismatches/"

// KindMismatchRecord is the JSON payload written to etcd when the drift
// reconciler detects a desired-kind/repo-kind mismatch for a package.
// The record is refreshed on every reconcile pass where the mismatch
// persists. When the mismatch is resolved the controller stops writing
// and the record becomes stale (the doctor ignores records older than
// kindMismatchStaleness).
type KindMismatchRecord struct {
	NodeID          string `json:"node_id"`
	PkgName         string `json:"pkg_name"`
	DesiredKind     string `json:"desired_kind"`
	RepoKind        string `json:"repo_kind"`
	DetectedAtUnix  int64  `json:"detected_at_unix"`
}

// writeKindMismatchRecord is the injectable function used by the reconciler
// to persist a kind mismatch. Tests replace this with a no-op or a spy.
var writeKindMismatchRecord = func(ctx context.Context, nodeID, pkgName, desiredKind, repoKind string) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("kind-mismatch: failed to get etcd client: %v", err)
		return
	}
	rec := KindMismatchRecord{
		NodeID:         nodeID,
		PkgName:        pkgName,
		DesiredKind:    desiredKind,
		RepoKind:       repoKind,
		DetectedAtUnix: time.Now().Unix(),
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	key := kindMismatchEtcdPrefix + nodeID + "/" + pkgName
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Put(wctx, key, string(b)); err != nil {
		log.Printf("kind-mismatch: failed to write record node=%s pkg=%s: %v", nodeID, pkgName, err)
	}
}
