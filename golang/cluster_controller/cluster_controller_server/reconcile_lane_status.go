// @awareness namespace=globular.platform
// @awareness component=platform_controller.reconciler
// @awareness file_role=reconcile_lane_status_tracking_and_transitions
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"time"
)

type reconcileLaneStatus struct {
	Lane             string `json:"lane"`
	Phase            string `json:"phase"` // OK | DEGRADED | TIMEOUT | BLOCKED
	Running          bool   `json:"running"`
	PreviousRunAlive bool   `json:"previous_run_active"`
	LastError        string `json:"last_error,omitempty"`
	UpdatedAtUnix    int64  `json:"updated_at_unix"`
}

func (srv *server) publishReconcileLaneStatus(ctx context.Context, lane string, st reconcileLaneStatus) {
	if lane == "" {
		return
	}
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return
	}
	st.Lane = lane
	st.UpdatedAtUnix = time.Now().Unix()
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _ = kv.Put(wctx, "/globular/controller/reconcile/lanes/"+lane, string(b))
}
