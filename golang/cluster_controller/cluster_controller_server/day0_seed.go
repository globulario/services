package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
	"github.com/google/uuid"
)

const (
	systemConfigKey          = "/globular/system/config"
	resourcesBootstrapMarker = "/globular/resources/bootstrap_marker"
	nodesBootstrapMarker     = "/globular/nodes/bootstrap_marker"
	scyllaBootstrapMarker    = "/globular/scylla/schema_guard/bootstrap_marker"
)

// startDay0SeedLoop continuously ensures baseline Day-0 cluster state exists
// in etcd. Leader-only writes prevent split-brain state publication.
func (srv *server) startDay0SeedLoop(ctx context.Context) {
	safeGoTracked("day0-seed", 30*time.Second, func(h *globular_service.SubsystemHandle) {
		// Run one pass quickly at startup so fresh Day-0 clusters don't sit in
		// contradictory "services running but critical state missing" mode.
		srv.seedDay0CriticalState(ctx)
		h.Tick()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.seedDay0CriticalState(ctx)
				h.Tick()
			}
		}
	})
}

func (srv *server) seedDay0CriticalState(ctx context.Context) {
	if !srv.isLeader() {
		return
	}
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		// Fallback to config-package client — same path used by the collector.
		// srv.kv/etcdClient may be nil if the dedicated connection failed at startup
		// while the controller still functions via config.GetEtcdClient().
		c, err := config.GetEtcdClient()
		if err != nil {
			log.Printf("day0-seed: no etcd client available, skipping: %v", err)
			return
		}
		kv = c
	}

	wctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	srv.ensureSystemConfigKey(wctx, kv)
	srv.ensureClusterMembershipID(wctx, kv)
	srv.ensureCriticalPrefixMarkers(wctx, kv)

	// Seed ingress/objectstore/CA via existing canonical publishers.
	srv.lock("seedDay0CriticalState")
	_ = srv.persistStateLocked(true)
	srv.unlock()
}

func (srv *server) ensureSystemConfigKey(ctx context.Context, kv kvClient) {
	resp, err := kv.Get(ctx, systemConfigKey)
	if err != nil || len(resp.Kvs) > 0 {
		return
	}

	cfg, err := config.GetLocalConfig(false)
	if err != nil {
		// Fallback: publish a minimal system config so Day-0 has authoritative
		// cluster identity even before full config materialization.
		cfg = map[string]interface{}{
			"Name":    srv.cfg.ClusterDomain,
			"Domain":  srv.cfg.ClusterDomain,
			"Version": Version,
		}
		log.Printf("day0-seed: local config unavailable, seeding minimal %s: %v", systemConfigKey, err)
	}
	// Keep this key lightweight and stable.
	delete(cfg, "Services")

	b, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	if _, err := kv.Put(ctx, systemConfigKey, string(b)); err != nil {
		log.Printf("day0-seed: failed to seed %s: %v", systemConfigKey, err)
		return
	}
	log.Printf("day0-seed: seeded %s", systemConfigKey)
}

// ensureClusterMembershipID mints the cluster's opaque MEMBERSHIP UUID exactly
// once, into the controller-owned key /globular/system/cluster/id. This is the
// canonical membership identity, deliberately distinct from the cluster domain
// (the domain remains the DNS/storage/workflow namespace). See
// config.ClusterMembershipIDKey and docs/design/cluster-id-minted-uuid-migration.md.
//
// Mint-once + immutable: if a non-empty value already exists it is left
// untouched (never overwritten). Idempotent — safe to run on every Day-0 seed
// pass (day0_day1_are_repeatable_ceremonies). Leader-only (caller-gated).
//
// This step is ADDITIVE: it makes the true identity exist and become readable.
// It does NOT change any validation — membership auth still uses the domain
// until the Phase-2 dual-accept airlock.
func (srv *server) ensureClusterMembershipID(ctx context.Context, kv kvClient) {
	resp, err := kv.Get(ctx, config.ClusterMembershipIDKey)
	if err != nil {
		log.Printf("day0-seed: cannot read %s: %v", config.ClusterMembershipIDKey, err)
		return
	}
	id := ""
	if len(resp.Kvs) > 0 {
		id = strings.TrimSpace(string(resp.Kvs[0].Value))
	}
	if id == "" {
		// Absent — mint once. (An existing non-empty value is immutable: never
		// overwritten, only cached below.)
		id = uuid.NewString()
		if _, err := kv.Put(ctx, config.ClusterMembershipIDKey, id); err != nil {
			log.Printf("day0-seed: failed to mint %s: %v", config.ClusterMembershipIDKey, err)
			return
		}
		log.Printf("day0-seed: minted cluster membership id %s at %s", id, config.ClusterMembershipIDKey)
	}
	// Cache the authoritative membership UUID into controller state so identity
	// readers (join gate, membership records, GetClusterInfo) use it without a
	// per-read etcd call. The domain is never a membership credential.
	srv.lock("ensureClusterMembershipID")
	if srv.state != nil && srv.state.ClusterUID != id {
		srv.state.ClusterUID = id
	}
	srv.unlock()
}

func (srv *server) ensureCriticalPrefixMarkers(ctx context.Context, kv kvClient) {
	ensureMarker := func(prefix, marker string) {
		_, _ = prefix, marker
		resp, err := kv.Get(ctx, marker)
		if err != nil || len(resp.Kvs) > 0 {
			return
		}
		payload := map[string]interface{}{
			"source":      "cluster-controller",
			"seed_reason": "day0_bootstrap",
			"written_at":  time.Now().UTC().Format(time.RFC3339),
		}
		b, _ := json.Marshal(payload)
		if _, err := kv.Put(ctx, marker, string(b)); err != nil {
			log.Printf("day0-seed: failed to seed marker %s: %v", marker, err)
			return
		}
		log.Printf("day0-seed: seeded marker %s", marker)
	}

	ensureMarker("/globular/resources/", resourcesBootstrapMarker)
	ensureMarker("/globular/nodes/", nodesBootstrapMarker)
	ensureMarker("/globular/scylla/schema_guard/", scyllaBootstrapMarker)
}
