// @awareness namespace=globular.platform
// @awareness component=platform_controller.scylla.stale_purger
// @awareness file_role=stale_closed_service_instance_etcd_purger
// @awareness implements=globular.platform:intent.health.requires_fresh_evidence
// @awareness implements=globular.platform:intent.infrastructure.scylladb.quorum_localdb_for_control_plane_state
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	instanceScanInterval = 5 * time.Minute
	staleInstanceMinAge  = 10 * time.Minute
	servicesEtcdPrefix   = "/globular/services/"
)

// instanceRecord is the JSON shape written by PutInstance / service registration.
type instanceRecord struct {
	Address   string `json:"Address"`
	State     string `json:"State"`
	UpdatedAt int64  `json:"UpdatedAt"`
	Process   int    `json:"Process"`
}

// runStaleInstancePurger is a leader-only background loop that deletes stale
// "closed" service instance entries from etcd.
//
// The problem it solves: when a node shuts down it writes State="closed" to
// its instance key. If the node never rejoins, that closed entry persists in
// etcd. Because etcd selects the most-recently-modified key per service, the
// closed entry (which has a high mod_revision from the shutdown write) can
// outrank a live "running" entry on another node, causing all service
// resolution to route to the dead address.
//
// A "closed" instance is eligible for deletion when:
//   - Its UpdatedAt field is at least staleInstanceMinAge (10 min) old
//   - The node whose IP appears in Address has a stale heartbeat (> heartbeatStaleThreshold)
//
// This is leader-only because instance deletion is a cluster mutation.
func (srv *server) runStaleInstancePurger(ctx context.Context) {
	// Startup delay: let the cluster settle and give nodes time to re-publish
	// their running state before we consider deleting anything.
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}

	ticker := time.NewTicker(instanceScanInterval)
	defer ticker.Stop()

	for {
		if srv.isLeader() {
			srv.purgeStaleInstances(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// purgeStaleInstances scans /globular/services/*/instances/* for closed entries
// whose node has a stale heartbeat, and deletes them.
// @awareness namespace=globular.platform
// @awareness component=platform_controller.scylla.stale_purger
// @awareness implements=globular.platform:intent.health.requires_fresh_evidence
// @awareness risk=high
func (srv *server) purgeStaleInstances(ctx context.Context) {
	if srv.etcdClient == nil {
		return
	}

	staleIPs := srv.staleNodeIPs()
	if len(staleIPs) == 0 {
		return
	}

	scanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := srv.etcdClient.Get(scanCtx, servicesEtcdPrefix, clientv3.WithPrefix())
	cancel()
	if err != nil {
		return
	}

	now := time.Now()
	deleted := 0

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.Contains(key, "/instances/") {
			continue
		}

		var inst instanceRecord
		if err := json.Unmarshal(kv.Value, &inst); err != nil {
			continue
		}

		if !strings.EqualFold(inst.State, "closed") {
			continue
		}

		age := now.Sub(time.Unix(inst.UpdatedAt, 0))
		if age < staleInstanceMinAge {
			continue
		}

		host := inst.Address
		if h, _, err := net.SplitHostPort(inst.Address); err == nil {
			host = h
		}
		if host == "" || !staleIPs[host] {
			continue
		}

		delCtx, delCancel := context.WithTimeout(ctx, 4*time.Second)
		_, delErr := srv.etcdClient.Delete(delCtx, key)
		delCancel()
		if delErr != nil {
			log.Printf("stale-instance-purger: delete %s: %v", key, delErr)
			continue
		}
		log.Printf("stale-instance-purger: deleted stale closed instance key=%s addr=%s age=%s",
			key, inst.Address, age.Truncate(time.Second))
		deleted++
	}

	if deleted > 0 {
		log.Printf("stale-instance-purger: removed %d stale closed instance(s)", deleted)
	}
}

// staleNodeIPs returns the set of IPs belonging to nodes whose heartbeat has
// not been seen for more than heartbeatStaleThreshold.
func (srv *server) staleNodeIPs() map[string]bool {
	srv.lock("staleNodeIPs")
	defer srv.unlock()
	stale := make(map[string]bool)
	for _, node := range srv.state.Nodes {
		if time.Since(node.LastSeen) <= heartbeatStaleThreshold {
			continue
		}
		for _, ip := range node.Identity.Ips {
			if ip = strings.TrimSpace(ip); ip != "" {
				stale[ip] = true
			}
		}
	}
	return stale
}
