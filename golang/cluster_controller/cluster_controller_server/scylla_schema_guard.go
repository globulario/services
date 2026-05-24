package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	scyllaKeyspaceRF = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_scylla_keyspace_rf",
		Help: "Current replication factor observed for keyspace.",
	}, []string{"keyspace"})
	scyllaKeyspaceRFRequired = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_scylla_keyspace_rf_required",
		Help: "Required replication factor for keyspace based on cluster size.",
	}, []string{"keyspace"})
	scyllaSchemaGuardViolation = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_scylla_schema_guard_violation",
		Help: "1 when keyspace RF is below policy, else 0.",
	}, []string{"keyspace"})
	scyllaSchemaGuardLastSuccessTS = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "globular_scylla_schema_guard_last_success_timestamp",
		Help: "Unix timestamp of the last successful schema guard pass.",
	})
	scyllaKeyspaceRepairRequired = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_scylla_keyspace_repair_required",
		Help: "1 when a nodetool repair is required for keyspace after RF was raised, else 0.",
	}, []string{"keyspace"})
)

var criticalScyllaKeyspaces = []string{
	"dns",
	"globular_projections",
	"workflow",
	"local_resource",
	"ai_memory",
	"globular_events",
	"repository",
}

const scyllaSchemaGuardEnforceRequestKey = "/globular/scylla/schema_guard/enforce_request"

func (srv *server) requestScyllaSchemaGuardEnforce(ctx context.Context, reason string) {
	if srv == nil {
		return
	}
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		reason = "manual"
	}
	payload := fmt.Sprintf("%d:%s", time.Now().Unix(), reason)
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := kv.Put(wctx, scyllaSchemaGuardEnforceRequestKey, payload); err != nil {
		log.Printf("scylla_schema_guard: enforce request write failed: %v", err)
	}
}

type schemaGuardStatus struct {
	Keyspace               string `json:"keyspace"`
	Strategy               string `json:"strategy"`
	CurrentRF              int    `json:"current_rf"`
	RequiredRF             int    `json:"required_rf"`
	Violation              bool   `json:"violation"`
	LastError              string `json:"last_error,omitempty"`
	UpdatedAtUnix          int64  `json:"updated_at_unix"`
	RepairRequired         bool   `json:"repair_required,omitempty"`
	RepairRequiredSinceUnix int64 `json:"repair_required_since_unix,omitempty"`
}

func (srv *server) startScyllaSchemaGuard(ctx context.Context) {
	safeGoTracked("scylla-schema-guard", 45*time.Second, func(h *globular_service.SubsystemHandle) {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					h.Tick()
					continue
				}
				if srv.consumeScyllaSchemaGuardEnforceRequest(ctx) {
					log.Printf("scylla_schema_guard: processing manual enforce request")
				}
				srv.runScyllaSchemaGuard(ctx)
				h.Tick()
			}
		}
	})
}

func (srv *server) consumeScyllaSchemaGuardEnforceRequest(ctx context.Context) bool {
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := kv.Get(rctx, scyllaSchemaGuardEnforceRequestKey)
	if err != nil || len(resp.Kvs) == 0 {
		return false
	}
	_, _ = kv.Delete(rctx, scyllaSchemaGuardEnforceRequestKey)
	return true
}

func (srv *server) runScyllaSchemaGuard(ctx context.Context) {
	hosts, err := config.GetScyllaHosts()
	if err != nil || len(hosts) == 0 {
		return
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = 9042
	cluster.Timeout = 5 * time.Second
	cluster.ConnectTimeout = 5 * time.Second
	cluster.Consistency = gocql.One
	session, err := cluster.CreateSession()
	if err != nil {
		log.Printf("scylla_schema_guard: open session failed: %v", err)
		return
	}
	defer session.Close()

	requiredRF := desiredRFForCluster(srv.storageControlPlaneNodeCount())
	for _, ks := range criticalScyllaKeyspaces {
		kctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		strategy, currentRF, rfMap, gerr := getKeyspaceReplication(kctx, session, ks)
		cancel()
		if gerr != nil {
			_ = srv.markSchemaGuardStatus(ctx, ks, schemaGuardStatus{
				Keyspace:      ks,
				RequiredRF:    requiredRF,
				Violation:     true,
				LastError:     gerr.Error(),
				UpdatedAtUnix: time.Now().Unix(),
			})
			continue
		}
		_ = rfMap
		scyllaKeyspaceRF.WithLabelValues(ks).Set(float64(currentRF))
		scyllaKeyspaceRFRequired.WithLabelValues(ks).Set(float64(requiredRF))
		violation := currentRF < requiredRF && requiredRF > 1
		if violation {
			scyllaSchemaGuardViolation.WithLabelValues(ks).Set(1)
			kctx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
			aerr := ensureKeyspaceRF(kctx2, session, ks, strategy, requiredRF)
			cancel2()
			if aerr != nil {
				_ = srv.markSchemaGuardStatus(ctx, ks, schemaGuardStatus{
					Keyspace:      ks,
					Strategy:      strategy,
					CurrentRF:     currentRF,
					RequiredRF:    requiredRF,
					Violation:     true,
					LastError:     aerr.Error(),
					UpdatedAtUnix: time.Now().Unix(),
				})
				continue
			}
			log.Printf("CRITICAL: scylla_schema_guard: keyspace %s RF raised %d→%d — REPAIR REQUIRED: run 'nodetool repair %s' on all storage nodes", ks, currentRF, requiredRF, ks)
			scyllaKeyspaceRepairRequired.WithLabelValues(ks).Set(1)
			_ = srv.markSchemaGuardStatus(ctx, ks, schemaGuardStatus{
				Keyspace:                ks,
				Strategy:                strategy,
				CurrentRF:               requiredRF,
				RequiredRF:              requiredRF,
				Violation:               false,
				RepairRequired:          true,
				RepairRequiredSinceUnix: time.Now().Unix(),
				UpdatedAtUnix:           time.Now().Unix(),
			})
			continue
		} else {
			scyllaSchemaGuardViolation.WithLabelValues(ks).Set(0)
			scyllaKeyspaceRepairRequired.WithLabelValues(ks).Set(0)
		}
		_ = srv.markSchemaGuardStatus(ctx, ks, schemaGuardStatus{
			Keyspace:      ks,
			Strategy:      strategy,
			CurrentRF:     currentRF,
			RequiredRF:    requiredRF,
			Violation:     violation,
			UpdatedAtUnix: time.Now().Unix(),
		})
	}
	scyllaSchemaGuardLastSuccessTS.Set(float64(time.Now().Unix()))
}

func getKeyspaceReplication(ctx context.Context, session *gocql.Session, keyspace string) (string, int, map[string]int, error) {
	var replication map[string]string
	q := session.Query(`SELECT replication FROM system_schema.keyspaces WHERE keyspace_name = ?`, keyspace).WithContext(ctx).Consistency(gocql.One)
	if err := q.Scan(&replication); err != nil {
		return "", 0, nil, err
	}
	strategy := replication["class"]
	if idx := strings.LastIndex(strategy, "."); idx >= 0 {
		strategy = strategy[idx+1:]
	}
	rfMap := map[string]int{}
	currentRF := 0
	if v := replication["replication_factor"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			currentRF = n
		}
	}
	for k, v := range replication {
		if k == "class" || k == "replication_factor" {
			continue
		}
		if n, err := strconv.Atoi(v); err == nil {
			rfMap[k] = n
			if n > currentRF {
				currentRF = n
			}
		}
	}
	return strategy, currentRF, rfMap, nil
}

func desiredRFForCluster(storageNodes int) int {
	switch {
	case storageNodes <= 1:
		return 1
	case storageNodes == 2:
		return 2
	default:
		return 3
	}
}

func ensureKeyspaceRF(ctx context.Context, session *gocql.Session, keyspace, strategy string, desiredRF int) error {
	switch strings.ToLower(strategy) {
	case "networktopologystrategy":
		// Without DC topology in state, enforce a single-DC minimum map.
		cql := fmt.Sprintf("ALTER KEYSPACE %s WITH REPLICATION = {'class':'NetworkTopologyStrategy','dc1':%d}", keyspace, desiredRF)
		return session.Query(cql).WithContext(ctx).Consistency(gocql.Quorum).Exec()
	default:
		cql := fmt.Sprintf("ALTER KEYSPACE %s WITH REPLICATION = {'class':'SimpleStrategy','replication_factor':%d}", keyspace, desiredRF)
		return session.Query(cql).WithContext(ctx).Consistency(gocql.Quorum).Exec()
	}
}

func (srv *server) markSchemaGuardStatus(ctx context.Context, keyspace string, st schemaGuardStatus) error {
	if srv.etcdClient == nil {
		return nil
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	k := "/globular/scylla/schema_guard/" + keyspace
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = srv.etcdClient.Put(wctx, k, string(b))
	return err
}

func (srv *server) storageControlPlaneNodeCount() int {
	srv.lock("scylla-schema-guard-count")
	defer srv.unlock()
	if srv.state == nil {
		return 1
	}
	count := 0
	for _, n := range srv.state.Nodes {
		if n == nil {
			continue
		}
		if n.Status == "removed" || n.Status == "blocked" || n.Status == "unreachable" {
			continue
		}
		profiles := map[string]bool{}
		for _, p := range n.Profiles {
			profiles[p] = true
		}
		if profiles["storage"] || profiles["control-plane"] {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}
