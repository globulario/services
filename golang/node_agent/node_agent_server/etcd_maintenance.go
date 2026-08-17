package main

// etcd_maintenance.go — periodic local etcd defragmentation.
//
// WHY THIS EXISTS
//
// etcd's backend file only ever grows. auto-compaction (configured here as
// periodic with 1h retention) discards old MVCC revisions, but that only marks
// their pages free INSIDE the file — the file itself never shrinks. Without a
// defragmentation the high-water mark is permanent, so the file ratchets up at
// the cluster's write rate until it reaches quota-backend-bytes, at which point
// etcd raises NOSPACE and goes read-only: no desired state can be written, no
// workflow dispatched, no service registered.
//
// Nothing in this codebase ever defragmented. etcd_lifecycle.go DETECTS the
// NOSPACE alarm and reports INFRA_DEGRADED, but detection without remediation
// only means the cluster explains its own death.
//
// MEASURED, not assumed (5-node simulation cluster, 2026-08-17):
//
//	write rate      46,200 revisions/hour, ~55 MB/hour of backend growth
//	live data       1.7 MB   (1018 keys — the working set is tiny)
//	backend file    2,146,975,744 bytes, i.e. 99% of the 2 GiB default quota
//	after defrag    1.7 MB on every member — ~97% of the file was free space
//	uptime at NOSPACE   46 hours
//
// 2 GiB / 55 MB per hour is about 39 hours. The cluster had been up 46. This is
// not a leak and not a workload spike: any Globular cluster reaches NOSPACE on
// a timer, and the only reason it is not seen sooner is that clusters get
// restarted for other reasons first.
//
// This also corrects a misdiagnosis worth recording. The same outage was first
// blamed on write volume, and cluster-controller@1.2.317 shipped an
// optimization that stopped persisting node liveness — which caused a second,
// worse outage. The arithmetic never supported it: with 1h retention the
// controller's ~78 KB blob at ~4 writes/min contributes ~30 MB of history,
// which cannot fill 2 GiB. The problem was never how much was written; it was
// that nothing ever gave the space back.
//
// SAFETY
//
// Defrag blocks the member it runs on for the duration — on a multi-GB file
// that can be minutes. Defragmenting several members at once would take out
// quorum, so this pass:
//
//   - operates ONLY on this node's own member, through a client pinned to the
//     local endpoint (infra.node_specific_truth_must_be_observed_via_node_local_client),
//   - runs at most one member per interval cluster-wide, by round-robin over
//     the sorted member list — deterministic, needs no coordination, and
//     crucially needs no etcd WRITE, so it still works when the backend is
//     already full and a lock could not be acquired,
//   - refuses to start unless the cluster has a leader and every member is
//     currently reachable, so a defrag never compounds an existing degradation,
//   - and only acts when there is enough reclaimable space to be worth the
//     pause (destructive_actions.require_explicit_guard).
//
// This performs no key mutation: it rewrites the local backend file only, and
// changes no cluster state. It is therefore local storage maintenance rather
// than a state transition needing a workflow instance.

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// etcdMaintenanceSlotWidth is how long one member's turn lasts. With
	// round-robin over N members, any one member is eligible every N slots —
	// for a 5-node cluster at 10 minutes, roughly every 50 minutes, which at
	// the measured 55 MB/hour keeps the file far below any sane quota.
	etcdMaintenanceSlotWidth = 10 * time.Minute

	// etcdMaintenanceInterval is how often a node EVALUATES whether the current
	// slot is its turn. It must be shorter than the slot width, and this is not
	// a tuning preference — it is a correctness requirement.
	//
	// The first version used one value for both. Each node then sampled the
	// slot sequence at a fixed phase determined by when its node-agent booted,
	// because the sampling period and the slot period were identical: node j
	// evaluates at boot_j + k*10min forever, so it lands on the same slot number
	// every time. A node whose boot phase did not coincide with its own index
	// was therefore eligible NEVER, not merely rarely.
	//
	// Measured on the 5-node simulation, 2026-08-17: the backend was inflated to
	// 467,714,048 bytes — well past the 256 MB floor, with ~440 MB reclaimable —
	// and across a 13-minute window covering two full slots, zero defrags ran on
	// any node. Sampling ten times per slot means every node observes its own
	// slot regardless of boot phase.
	etcdMaintenanceInterval = 1 * time.Minute

	// etcdDefragMinFileBytes — below this the file is not a threat and the
	// pause is not worth taking.
	etcdDefragMinFileBytes = 256 << 20 // 256 MB

	// etcdDefragMinReclaimBytes — the pause is only justified when this much
	// would actually come back. dbSize-dbSizeInUse is etcd's own estimate of
	// free pages, so this is measured, not guessed.
	etcdDefragMinReclaimBytes = 64 << 20 // 64 MB

	// etcdDefragTimeout bounds a single defrag. A multi-GB backend can take
	// minutes; anything longer than this means the member is in trouble for a
	// different reason and should be left alone.
	etcdDefragTimeout = 10 * time.Minute

	etcdMaintenanceDialTimeout = 5 * time.Second
)

// StartEtcdMaintenance launches the maintenance loop if this node runs an etcd
// member. etcd runs on ALL nodes, so this is every node — but the guard keeps
// the loop from spinning on a node where the local member is not reachable.
func (srv *NodeAgentServer) StartEtcdMaintenance(ctx context.Context) {
	// The VIP floats between control-plane nodes and etcd does not bind to it,
	// so resolving "this node" through it would point maintenance at whichever
	// machine currently holds the VIP
	// (netutil.identity_getter_must_express_vip_ambiguity). Use the stable
	// interface address, the same way the infra health probe does.
	vip := srv.lookupIngressVIP()
	localIP := config.GetLocalInterfaceIPv4(vip)
	if localIP == "" {
		localIP = config.GetRoutableIPv4()
	}
	if localIP == "" {
		slog.Warn("etcd.maintenance_not_started",
			"reason", "could not resolve this node's stable interface address")
		return
	}

	localClientURL := fmt.Sprintf("https://%s:2379", localIP)
	slog.Info("etcd.maintenance_started",
		"endpoint", localClientURL,
		"interval", etcdMaintenanceInterval.String(),
		"min_file_bytes", int64(etcdDefragMinFileBytes),
		"min_reclaim_bytes", int64(etcdDefragMinReclaimBytes))

	go srv.runEtcdMaintenanceLoop(ctx, localClientURL)
}

// runEtcdMaintenanceLoop evaluates the local member on every tick for the life
// of the process. It never returns until ctx is done.
func (srv *NodeAgentServer) runEtcdMaintenanceLoop(ctx context.Context, localClientURL string) {
	ticker := time.NewTicker(etcdMaintenanceInterval)
	defer ticker.Stop()

	// The loop now evaluates several times per slot, so it must remember which
	// slot it last acted in or it would defrag repeatedly inside its own turn.
	// -1 is "has not acted", which no real slot number can collide with.
	lastActedSlot := int64(-1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Errors are logged and the loop continues: maintenance is
			// best-effort and must never take the agent down. Deliberately not
			// spawned as a goroutine per tick
			// (error_path.no_unbounded_fire_and_forget_goroutine) — a slow
			// defrag delays the next evaluation, which is the correct
			// behaviour.
			actedSlot, err := srv.etcdMaintenancePass(ctx, localClientURL, lastActedSlot)
			if err != nil {
				slog.Warn("etcd.maintenance_pass_failed", "err", err)
			}
			if actedSlot >= 0 {
				lastActedSlot = actedSlot
			}
		}
	}
}

// etcdMaintenancePass runs one evaluation. It defragments the local member only
// when every guard passes, and reports what it decided either way — a pass that
// silently does nothing is indistinguishable from one that is not running.
func (srv *NodeAgentServer) etcdMaintenancePass(
	ctx context.Context, localClientURL string, lastActedSlot int64,
) (actedSlot int64, err error) {
	endpoint := etcdHostPort(localClientURL)
	if endpoint == "" {
		return -1, fmt.Errorf("etcd maintenance: could not derive host:port from local client URL %q", localClientURL)
	}

	tlsCfg, err := config.GetEtcdTLS()
	if err != nil {
		return -1, fmt.Errorf("etcd maintenance: TLS unavailable: %w", err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: etcdMaintenanceDialTimeout,
		TLS:         tlsCfg,
		Context:     ctx,
	})
	if err != nil {
		return -1, fmt.Errorf("etcd maintenance: dial %s: %w", endpoint, err)
	}
	defer func() { _ = cli.Close() }()

	statusCtx, cancel := context.WithTimeout(ctx, etcdMaintenanceDialTimeout)
	st, err := cli.Status(statusCtx, endpoint)
	cancel()
	if err != nil {
		return -1, fmt.Errorf("etcd maintenance: status %s: %w", endpoint, err)
	}

	// A member without a leader, or still catching up as a learner, must not be
	// paused for maintenance.
	if st.Leader == 0 {
		slog.Debug("etcd.maintenance_skipped", "reason", "no raft leader", "endpoint", endpoint)
		return -1, nil
	}
	if st.IsLearner {
		slog.Debug("etcd.maintenance_skipped", "reason", "member is a learner", "endpoint", endpoint)
		return -1, nil
	}

	members, err := srv.etcdMemberIDs(ctx, cli)
	if err != nil {
		return -1, fmt.Errorf("etcd maintenance: member list: %w", err)
	}
	if len(members) == 0 {
		return -1, fmt.Errorf("etcd maintenance: member list is empty")
	}

	// Round-robin: exactly one member is eligible per interval, cluster-wide.
	// Derived from wall-clock and the sorted member list, so every node reaches
	// the same conclusion without talking to any other node and without writing
	// to etcd — which matters because when the backend is full, writes are
	// exactly what is unavailable.
	myIndex := -1
	for i, id := range members {
		if id == st.Header.MemberId {
			myIndex = i
			break
		}
	}
	if myIndex < 0 {
		return -1, fmt.Errorf("etcd maintenance: local member %x not present in member list", st.Header.MemberId)
	}
	// Slot number is absolute, not modulo — so "have I already acted in THIS
	// turn" is answerable across evaluations. Eligibility is the modulo.
	slotNum := time.Now().Unix() / int64(etcdMaintenanceSlotWidth/time.Second)
	if slotNum%int64(len(members)) != int64(myIndex) {
		slog.Debug("etcd.maintenance_not_my_turn",
			"slot", slotNum, "my_index", myIndex, "members", len(members))
		return -1, nil
	}
	if slotNum == lastActedSlot {
		slog.Debug("etcd.maintenance_already_acted_this_slot", "slot", slotNum)
		return -1, nil
	}

	// Measure before deciding (diagnostics.must_measure_reality). dbSizeInUse
	// is what the data actually occupies; the difference is free pages that a
	// defrag returns to the filesystem.
	reclaimable := st.DbSize - st.DbSizeInUse
	if st.DbSize < etcdDefragMinFileBytes {
		slog.Debug("etcd.maintenance_skipped",
			"reason", "backend below size floor",
			"db_size_bytes", st.DbSize, "floor_bytes", int64(etcdDefragMinFileBytes))
		return -1, nil
	}
	if reclaimable < etcdDefragMinReclaimBytes {
		slog.Debug("etcd.maintenance_skipped",
			"reason", "not enough reclaimable space to justify pausing the member",
			"reclaimable_bytes", reclaimable, "min_bytes", int64(etcdDefragMinReclaimBytes))
		return -1, nil
	}

	// Never defrag into an already-degraded cluster: pausing this member on top
	// of another one being down is how maintenance causes the outage it exists
	// to prevent.
	if unhealthy, err := srv.etcdUnhealthyMembers(ctx, tlsCfg, members, cli); err != nil {
		return -1, fmt.Errorf("etcd maintenance: member health check: %w", err)
	} else if unhealthy > 0 {
		slog.Warn("etcd.maintenance_deferred",
			"reason", "one or more members are unreachable — not pausing another",
			"unhealthy_members", unhealthy)
		return -1, nil
	}

	slog.Info("etcd.defrag_starting",
		"endpoint", endpoint,
		"db_size_bytes", st.DbSize,
		"db_size_in_use_bytes", st.DbSizeInUse,
		"reclaimable_bytes", reclaimable)

	defragCtx, defragCancel := context.WithTimeout(ctx, etcdDefragTimeout)
	_, err = cli.Defragment(defragCtx, endpoint)
	defragCancel()
	if err != nil {
		return -1, fmt.Errorf("etcd maintenance: defragment %s: %w", endpoint, err)
	}

	// Report the outcome measured, not the outcome intended.
	afterCtx, afterCancel := context.WithTimeout(ctx, etcdMaintenanceDialTimeout)
	after, afterErr := cli.Status(afterCtx, endpoint)
	afterCancel()
	if afterErr != nil {
		slog.Info("etcd.defrag_complete",
			"endpoint", endpoint, "post_status", "unavailable", "err", afterErr)
		return slotNum, nil
	}
	slog.Info("etcd.defrag_complete",
		"endpoint", endpoint,
		"db_size_before_bytes", st.DbSize,
		"db_size_after_bytes", after.DbSize,
		"freed_bytes", st.DbSize-after.DbSize)
	return slotNum, nil
}

// etcdMemberIDs returns every member ID, sorted, so that all nodes derive the
// same round-robin ordering from the same cluster.
func (srv *NodeAgentServer) etcdMemberIDs(ctx context.Context, cli *clientv3.Client) ([]uint64, error) {
	listCtx, cancel := context.WithTimeout(ctx, etcdMaintenanceDialTimeout)
	defer cancel()

	resp, err := cli.MemberList(listCtx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(resp.Members))
	for _, m := range resp.Members {
		// Learners are not voters and must not consume a maintenance slot.
		if m.IsLearner {
			continue
		}
		ids = append(ids, m.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// etcdUnhealthyMembers counts members that do not answer a status call. Each is
// asked on its own pinned endpoint rather than through a pooled client, so the
// answer describes that member and not whichever one a pool happened to pick.
func (srv *NodeAgentServer) etcdUnhealthyMembers(
	ctx context.Context,
	tlsCfg *tls.Config,
	memberIDs []uint64,
	local *clientv3.Client,
) (int, error) {
	listCtx, cancel := context.WithTimeout(ctx, etcdMaintenanceDialTimeout)
	resp, err := local.MemberList(listCtx)
	cancel()
	if err != nil {
		return 0, err
	}

	unhealthy := 0
	for _, m := range resp.Members {
		if m.IsLearner || len(m.ClientURLs) == 0 {
			continue
		}
		ep := etcdHostPort(m.ClientURLs[0])
		if ep == "" {
			continue
		}
		peer, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{ep},
			DialTimeout: etcdMaintenanceDialTimeout,
			TLS:         tlsCfg,
			Context:     ctx,
		})
		if err != nil {
			unhealthy++
			continue
		}
		stCtx, stCancel := context.WithTimeout(ctx, etcdMaintenanceDialTimeout)
		_, err = peer.Status(stCtx, ep)
		stCancel()
		_ = peer.Close()
		if err != nil {
			unhealthy++
		}
	}
	return unhealthy, nil
}
