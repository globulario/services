// @awareness namespace=globular.platform
// @awareness component=platform_controller.leader_election
// @awareness file_role=leader_pending_state_etcd_sync
// @awareness implements=globular.platform:intent.etcd.is_source_of_truth
// @awareness risk=critical
package main

import (
	"context"
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/config"
)

const leaderPendingUpdateKey = "/globular/controller/leader_pending_update"

// LeaderPendingUpdateRecord is written to etcd while the controller leader
// cannot resign because no follower is at the target build. The record is
// refreshed every reconcile pass (~30 s). When the condition resolves — a
// safe successor is found or the leader updates itself — the record is
// deleted.
//
// StuckSinceUnix is set on first detection and held constant while stuck,
// allowing the doctor to escalate severity when the leader has been blocked
// for more than pendingUpdateEscalateAfter.
type LeaderPendingUpdateRecord struct {
	LeaderNodeID   string            `json:"leader_node_id"`
	CurrentVersion string            `json:"current_version"`
	TargetVersion  string            `json:"target_version"`
	FollowersTotal int               `json:"followers_total"`
	BlockedReasons map[string]string `json:"blocked_reasons"`
	StuckSinceUnix int64             `json:"stuck_since_unix"`
	DetectedAtUnix int64             `json:"detected_at_unix"`
}

// leaderStuckSince is set on first detection and cleared when the condition
// resolves. 0 means not currently stuck.
var leaderStuckSince atomic.Int64

// writeLeaderPendingUpdate persists the stuck record to etcd. Tests replace
// this with a spy or no-op.
var writeLeaderPendingUpdate = func(ctx context.Context, rec LeaderPendingUpdateRecord) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("leader-pending-update: failed to get etcd client: %v", err)
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Put(wctx, leaderPendingUpdateKey, string(b)); err != nil {
		log.Printf("leader-pending-update: failed to write record: %v", err)
	}
}

// clearLeaderPendingUpdate removes the stuck record from etcd when the
// condition is resolved. Tests replace this with a spy or no-op.
var clearLeaderPendingUpdate = func(ctx context.Context) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Delete(wctx, leaderPendingUpdateKey); err != nil {
		log.Printf("leader-pending-update: failed to clear record: %v", err)
	}
}
