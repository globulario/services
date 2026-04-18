// Package main implements the Workflow gRPC service backed by ScyllaDB.
// It provides cluster-scoped, persistent reconciliation workflow tracing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/tls"
	"crypto/x509"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dephealth"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/workflow/workflowpb"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	Version   = ""
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var (
	defaultPort  = 10220
	defaultProxy = 10221
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// ---------------------------------------------------------------------------
// Service definition
// ---------------------------------------------------------------------------

type server struct {
	// Globular service metadata
	Id                 string
	Mac                string
	Name               string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	AllowAllOrigins    bool
	AllowedOrigins     string
	Protocol           string
	Domain             string
	Address            string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	State              string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	Checksum           string
	Plaform            string
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64

	// gRPC
	workflowpb.UnimplementedWorkflowServiceServer
	grpcServer *grpc.Server

	// ScyllaDB
	ScyllaHosts             []string
	ScyllaPort              int
	ScyllaReplicationFactor int
	session                 *gocql.Session

	// Live watchers
	watchersMu sync.RWMutex
	watchers   map[string][]chan *workflowpb.WorkflowEventEnvelope // run_id → channels
	nodeWatch  map[string][]chan *workflowpb.WorkflowEventEnvelope // node_id → channels

	// Executor lease manager for HA run ownership (HA-4).
	leaseManager *executorLeaseManager

	// AI-memory client for incident projection (AL-1).
	aiMemoryClient    ai_memorypb.AiMemoryServiceClient
	incidentDedupeMu  sync.RWMutex
	incidentDedupeMap map[string]time.Time

	// Dependency health watchdog (gates RPCs when ScyllaDB is down).
	depHealth *dephealth.Watchdog

	// Metrics bookkeeping (low cardinality, held locally)
	metricsMu    sync.Mutex
	runStart     map[string]time.Time // run_id -> start time
	lastStepUnix time.Time
}

// ---------------------------------------------------------------------------
// Globular service contract
// ---------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(address string)        { srv.Address = address }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		srv.closeScylla()
	}
	srv.Process = pid
}
func (srv *server) GetProxyProcess() int              { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)           { srv.ProxyProcess = pid }
func (srv *server) GetState() string                  { return srv.State }
func (srv *server) SetState(state string)             { srv.State = state }
func (srv *server) GetLastError() string              { return srv.LastError }
func (srv *server) SetLastError(err string)           { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)          { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                 { return srv.ModTime }
func (srv *server) GetId() string                     { return srv.Id }
func (srv *server) SetId(id string)                   { srv.Id = id }
func (srv *server) GetName() string                   { return srv.Name }
func (srv *server) SetName(name string)               { srv.Name = name }
func (srv *server) GetMac() string                    { return srv.Mac }
func (srv *server) SetMac(mac string)                 { srv.Mac = mac }
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)     { srv.Keywords = keywords }
func (srv *server) Dist(path string) (string, error)  { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	for _, d := range srv.Dependencies {
		if d == dep {
			return
		}
	}
	srv.Dependencies = append(srv.Dependencies, dep)
}
func (srv *server) GetChecksum() string                      { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)              { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                      { return srv.Plaform }
func (srv *server) SetPlatform(platform string)              { srv.Plaform = platform }
func (srv *server) GetRepositories() []string                { return srv.Repositories }
func (srv *server) SetRepositories(v []string)               { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string                 { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)                { srv.Discoveries = v }
func (srv *server) GetPath() string                          { return srv.Path }
func (srv *server) SetPath(path string)                      { srv.Path = path }
func (srv *server) GetProto() string                         { return srv.Proto }
func (srv *server) SetProto(proto string)                    { srv.Proto = proto }
func (srv *server) GetPort() int                             { return srv.Port }
func (srv *server) SetPort(port int)                         { srv.Port = port }
func (srv *server) GetProxy() int                            { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                       { srv.Proxy = proxy }
func (srv *server) GetProtocol() string                      { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)              { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                 { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)                { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)               { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string                        { return srv.Domain }
func (srv *server) SetDomain(domain string)                  { srv.Domain = domain }
func (srv *server) GetTls() bool                             { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                       { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string            { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)          { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                      { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)              { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                       { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)                { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                       { return srv.Version }
func (srv *server) SetVersion(version string)                { srv.Version = version }
func (srv *server) GetPublisherID() string                   { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)                  { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }
func (srv *server) GetGrpcServer() *grpc.Server              { return srv.grpcServer }
func (srv *server) Save() error                              { return globular.SaveService(srv) }
func (srv *server) StartService() error                      { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error                       { return globular.StopService(srv, srv.grpcServer) }
func (srv *server) RolesDefault() []resourcepb.Role          { return []resourcepb.Role{} }

// requireHealthy gates RPCs — returns codes.Unavailable when dependencies are down.
func (srv *server) requireHealthy() error {
	if srv.depHealth == nil {
		return nil
	}
	return srv.depHealth.RequireHealthy()
}

// ---------------------------------------------------------------------------
// ScyllaDB connection
// ---------------------------------------------------------------------------

func (srv *server) connectScylla() error {
	if srv.session != nil {
		return nil
	}

	hosts := srv.ScyllaHosts
	if len(hosts) == 0 {
		etcdHosts, err := config.GetScyllaHosts()
		if err != nil {
			return fmt.Errorf("scylla hosts: %w", err)
		}
		hosts = etcdHosts
	}
	port := srv.ScyllaPort
	if port == 0 {
		port = 9042
	}
	rf := srv.ScyllaReplicationFactor
	if rf == 0 {
		rf = 1
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = gocql.LocalOne
	cluster.Timeout = 15 * time.Second
	cluster.ConnectTimeout = 15 * time.Second
	cluster.ProtoVersion = 4
	cluster.DisableInitialHostLookup = true

	// Try connecting directly to the keyspace first (fast path for existing installs).
	cluster.Keyspace = workflowKeyspace
	session, err := cluster.CreateSession()
	if err != nil {
		// Keyspace may not exist yet — connect without keyspace and create it.
		cluster.Keyspace = ""
		session, err = cluster.CreateSession()
		if err != nil {
			return fmt.Errorf("scylla connect: %w", err)
		}
		cql := fmt.Sprintf(createWorkflowKeyspaceCQL, rf)
		if err := session.Query(cql).Exec(); err != nil {
			session.Close()
			return fmt.Errorf("create keyspace: %w", err)
		}
		session.Close()
		// Reconnect with keyspace.
		cluster.Keyspace = workflowKeyspace
		session, err = cluster.CreateSession()
		if err != nil {
			return fmt.Errorf("scylla reconnect with keyspace: %w", err)
		}
	}

	// Check if tables already exist before running DDL (DDL can timeout on ScyllaDB 2025.3+).
	var tableCount int
	if err := session.Query(`SELECT count(*) FROM system_schema.tables WHERE keyspace_name = ?`, workflowKeyspace).Scan(&tableCount); err != nil {
		tableCount = 0
	}
	// Always run schema statements (all use CREATE TABLE IF NOT EXISTS), so
	// upgrades with new tables propagate to existing deployments without
	// requiring manual migration.
	for _, stmt := range schemaCQLStatements {
		if err := session.Query(stmt).Exec(); err != nil {
			session.Close()
			return fmt.Errorf("schema init: %w", err)
		}
	}
	_ = tableCount

	srv.session = session
	if err != nil {
		return fmt.Errorf("scylla connect (keyspace): %w", err)
	}

	logger.Info("ScyllaDB connected", "hosts", hosts, "keyspace", workflowKeyspace)
	return nil
}

func (srv *server) closeScylla() {
	if srv.session != nil {
		srv.session.Close()
		srv.session = nil
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}

	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	// ScyllaDB hosts MUST come from etcd (Tier-0 cluster key). The service
	// config may contain stale addresses from a previous boot; the cluster
	// key is the sole source of truth for infrastructure addresses.
	if hosts, err := config.GetScyllaHosts(); err == nil && len(hosts) > 0 {
		srv.ScyllaHosts = hosts
	} else if len(srv.ScyllaHosts) == 0 {
		return fmt.Errorf("scylla hosts unavailable (etcd key %s): %w",
			"/globular/cluster/scylla/hosts", err)
	}

	if err := srv.connectScylla(); err != nil {
		return fmt.Errorf("scylla init: %w", err)
	}

	// Dependency health watchdog — gates RPCs when ScyllaDB is unreachable.
	srv.depHealth = dephealth.NewWatchdog(logger,
		dephealth.Dep("scylladb", func(ctx context.Context) error {
			if srv.session == nil {
				return fmt.Errorf("scylladb not connected")
			}
			return srv.session.Query("SELECT now() FROM system.local").Consistency(gocql.One).Exec()
		}),
	)
	go srv.depHealth.Start(context.Background())

	// Initialize executor lease manager for HA run ownership.
	srv.leaseManager = newExecutorLeaseManager(srv)
	srv.leaseManager.StartOrphanScanner(context.Background())

	// AL-1: Connect to ai-memory for incident projection.
	// Best-effort — if ai-memory is unavailable, incidents are skipped.
	if memAddr := config.ResolveLocalServiceAddr("ai_memory.AiMemoryService"); memAddr != "" {
		dt := config.ResolveDialTarget(memAddr)
		if memConn, err := grpc.NewClient(dt.Address, grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				ServerName: dt.ServerName,
				RootCAs:    srv.loadCAPool(),
			}),
		)); err == nil {
			srv.aiMemoryClient = ai_memorypb.NewAiMemoryServiceClient(memConn)
			logger.Info("incident projection: ai-memory connected", "addr", dt.Address)
		} else {
			logger.Warn("incident projection: ai-memory unavailable", "err", err)
		}
	}

	// Import Day-0 install trace if the JSON log exists (idempotent).
	srv.importDay0Trace()

	// Close stale DISPATCHED runs that the node agent never finished
	// (e.g. workflow service was down when the node agent tried to call FinishRun).
	globular.RegisterSubsystem("reap-stale-runs", 5*time.Minute)
	go srv.reapStaleRuns()

	// Scan convergence telemetry and emit threshold events so ai-watcher
	// (or any subscriber) can react when step failure rates rise, drift
	// gets stuck, or periodic workflows stop firing.
	globular.RegisterSubsystem("telemetry-scan", 60*time.Second)
	go srv.scanTelemetryAndEmit()

	// Aggregate telemetry into operator-facing incidents per minute.
	// See docs/incidents-design.md.
	globular.RegisterSubsystem("incident-scanner", 60*time.Second)
	go srv.runIncidentScanner()

	return nil
}

// scanTelemetryAndEmit polls the convergence telemetry tables every minute
// and emits workflow.* events when thresholds cross. Idempotent per event_key
// (we remember the last emitted key and only fire on deltas).
func (srv *server) scanTelemetryAndEmit() {
	const (
		scanInterval      = 60 * time.Second
		minExecutions     = 5
		failureRateThresh = 0.10
		driftStuckThresh  = 3
	)
	// Per-workflow inactivity thresholds.
	noActivity := map[string]time.Duration{
		"cluster.reconcile": 5 * time.Minute,
	}

	// Last-fired-at per event key, to deduplicate noisy scans.
	lastFired := make(map[string]time.Time)
	const cooldown = 5 * time.Minute

	fire := func(key, topic string, payload map[string]interface{}) {
		if t, ok := lastFired[key]; ok && time.Since(t) < cooldown {
			return
		}
		lastFired[key] = time.Now()
		publishWorkflowEvent(topic, payload)
	}

	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	for range ticker.C {
		// We need a cluster_id. Lift it from any existing summary row.
		var clusterID string
		if err := srv.session.Query(
			`SELECT cluster_id FROM workflow_run_summaries LIMIT 1`,
		).Scan(&clusterID); err != nil || clusterID == "" {
			continue
		}

		// Step failure rates.
		iter := srv.session.Query(
			`SELECT workflow_name, step_id, total_executions, failure_count, last_error_message
			 FROM workflow_step_outcomes WHERE cluster_id=?`, clusterID,
		).Iter()
		for {
			var (
				wf, step, lastErr string
				total, fail       int64
			)
			if !iter.Scan(&wf, &step, &total, &fail, &lastErr) {
				break
			}
			if total < minExecutions || fail == 0 {
				continue
			}
			rate := float64(fail) / float64(total)
			if rate < failureRateThresh {
				continue
			}
			key := "step_fail:" + wf + "/" + step
			fire(key, "workflow.step.failures_high", map[string]interface{}{
				"cluster_id":       clusterID,
				"workflow_name":    wf,
				"step_id":          step,
				"failure_rate":     rate,
				"failure_count":    fail,
				"total_executions": total,
				"last_error":       lastErr,
			})
		}
		_ = iter.Close()

		// Drift stuck.
		iter2 := srv.session.Query(
			`SELECT drift_type, entity_ref, consecutive_cycles, chosen_workflow
			 FROM drift_unresolved WHERE cluster_id=?`, clusterID,
		).Iter()
		for {
			var (
				dType, eRef, chosen string
				cycles              int
			)
			if !iter2.Scan(&dType, &eRef, &cycles, &chosen) {
				break
			}
			if cycles < driftStuckThresh {
				continue
			}
			key := "drift_stuck:" + dType + "/" + eRef
			fire(key, "workflow.drift.stuck", map[string]interface{}{
				"cluster_id":         clusterID,
				"drift_type":         dType,
				"entity_ref":         eRef,
				"consecutive_cycles": cycles,
				"chosen_workflow":    chosen,
			})
		}
		_ = iter2.Close()

		// No activity on periodic workflows.
		for wfName, threshold := range noActivity {
			var lastFinished time.Time
			if err := srv.session.Query(
				`SELECT last_finished_at FROM workflow_run_summaries WHERE cluster_id=? AND workflow_name=?`,
				clusterID, wfName,
			).Scan(&lastFinished); err != nil {
				continue
			}
			if lastFinished.IsZero() {
				continue
			}
			age := time.Since(lastFinished)
			if age < threshold {
				continue
			}
			key := "no_activity:" + wfName
			fire(key, "workflow.no_activity", map[string]interface{}{
				"cluster_id":        clusterID,
				"workflow_name":     wfName,
				"age_seconds":       int64(age.Seconds()),
				"threshold_seconds": int64(threshold.Seconds()),
			})
		}
	}
}

// reapStaleRuns ensures every run has an explicit terminal outcome.
// Runs stuck in non-terminal states past the deadline are marked FAILED
// with reason "timeout". This prevents orphaned runs when the workflow
// engine crashes before FinishRun is called.
func (srv *server) reapStaleRuns() {
	const staleThreshold = 15 * time.Minute
	const sweepInterval = 5 * time.Minute

	for {
		if srv.session == nil {
			return
		}

		iter := srv.session.Query(`
			SELECT id, cluster_id, started_at, status, updated_at
			FROM workflow_runs LIMIT 500 ALLOW FILTERING`,
		).Iter()

		var (
			id, clusterID        string
			status               int
			startedAt, updatedAt time.Time
		)
		now := time.Now()
		reaped := 0
		for iter.Scan(&id, &clusterID, &startedAt, &status, &updatedAt) {
			s := workflowpb.RunStatus(status)
			// Skip terminal statuses — they're done.
			if isTerminalStatus(s) {
				continue
			}
			lastActivity := updatedAt
			if lastActivity.IsZero() {
				lastActivity = startedAt
			}
			if now.Sub(lastActivity) < staleThreshold {
				continue
			}
			// Mark as FAILED with timeout reason.
			srv.session.Query(`
				UPDATE workflow_runs SET status=?, failure_class=?, error_message=?, summary=?, finished_at=?, updated_at=?
				WHERE cluster_id=? AND started_at=? AND id=?`,
				int(workflowpb.RunStatus_RUN_STATUS_FAILED),
				int(workflowpb.FailureClass_FAILURE_CLASS_UNKNOWN),
				"timeout: no progress for "+staleThreshold.String(),
				"Reaped by watchdog: run stuck in "+s.String(),
				now, now,
				clusterID, startedAt, id,
			).Exec()
			reaped++
		}
		iter.Close()

		if reaped > 0 {
			slog.Warn("reaped stale runs", "reaped", reaped, "threshold", staleThreshold)
		}

		time.Sleep(sweepInterval)
	}
}

// isTerminalStatus returns true if the run status is a terminal outcome
// and should not be reaped or touched.
func isTerminalStatus(s workflowpb.RunStatus) bool {
	switch s {
	case workflowpb.RunStatus_RUN_STATUS_SUCCEEDED,
		workflowpb.RunStatus_RUN_STATUS_FAILED,
		workflowpb.RunStatus_RUN_STATUS_CANCELED,
		workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK,
		workflowpb.RunStatus_RUN_STATUS_SUPERSEDED:
		return true
	}
	return false
}

// isTerminalStepStatus returns true if the step status is a terminal outcome.
func isTerminalStepStatus(s workflowpb.StepStatus) bool {
	switch s {
	case workflowpb.StepStatus_STEP_STATUS_SUCCEEDED,
		workflowpb.StepStatus_STEP_STATUS_FAILED,
		workflowpb.StepStatus_STEP_STATUS_SKIPPED:
		return true
	}
	return false
}

// supersedePriorRuns finds non-terminal runs with the same correlation_id
// as the new run and marks them SUPERSEDED with superseded_by=newRunID.
// Called from StartRun so each correlation_id has at most one active run.
func (srv *server) supersedePriorRuns(clusterID, correlationID, newRunID string) {
	if srv.session == nil || correlationID == "" {
		return
	}
	// PageSize limits the scan to avoid O(N²) degradation after many retries.
	// Most runs will be terminal (SUPERSEDED/SUCCEEDED/FAILED); we only need
	// to find and mark the few non-terminal ones.
	iter := srv.session.Query(`
		SELECT id, started_at, status FROM workflow_runs
		WHERE cluster_id=? AND correlation_id=? ALLOW FILTERING`,
		clusterID, correlationID,
	).PageSize(100).Iter()

	var (
		id        string
		startedAt time.Time
		status    int
	)
	now := time.Now()
	superseded := 0
	for iter.Scan(&id, &startedAt, &status) {
		if id == newRunID {
			continue
		}
		if isTerminalStatus(workflowpb.RunStatus(status)) {
			continue
		}
		// Mark prior run as SUPERSEDED.
		srv.session.Query(`
			UPDATE workflow_runs SET status=?, superseded_by=?, finished_at=?, updated_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			int(workflowpb.RunStatus_RUN_STATUS_SUPERSEDED),
			newRunID, now, now,
			clusterID, startedAt, id,
		).Exec()
		superseded++
	}
	iter.Close()
	if superseded > 0 {
		slog.Info("superseded prior runs", "correlation_id", correlationID,
			"new_run_id", newRunID, "count", superseded)
	}
}

// ---------------------------------------------------------------------------
// Write RPCs
// ---------------------------------------------------------------------------

func (srv *server) StartRun(_ context.Context, req *workflowpb.StartRunRequest) (*workflowpb.WorkflowRun, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	run := req.GetRun()
	if run == nil {
		return nil, fmt.Errorf("run is required")
	}
	if run.Id == "" {
		run.Id = gocql.TimeUUID().String()
	}
	ctx := run.GetContext()
	if ctx == nil {
		return nil, fmt.Errorf("run.context is required")
	}
	now := time.Now()
	if run.StartedAt == nil {
		run.StartedAt = timestamppb.New(now)
	}
	run.UpdatedAt = timestamppb.New(now)

	// Supersede any non-terminal runs with the same correlation_id so we
	// don't have two active runs racing for the same operator story.
	if run.CorrelationId != "" {
		srv.supersedePriorRuns(ctx.ClusterId, run.CorrelationId, run.Id)
	}

	if err := srv.session.Query(`
		INSERT INTO workflow_runs (
			cluster_id, id, correlation_id, parent_run_id,
			node_id, node_hostname, component_name, component_kind, component_version,
			release_kind, release_object_id, desired_object_id,
			trigger_reason, status, current_actor, failure_class,
			summary, error_message, retry_count,
			acknowledged, acknowledged_by, acknowledged_at,
			started_at, updated_at, finished_at, workflow_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ctx.ClusterId, run.Id, run.CorrelationId, run.ParentRunId,
		ctx.NodeId, ctx.NodeHostname, ctx.ComponentName, int(ctx.ComponentKind), ctx.ComponentVersion,
		ctx.ReleaseKind, ctx.ReleaseObjectId, ctx.DesiredObjectId,
		int(run.TriggerReason), int(run.Status), int(run.CurrentActor), int(run.FailureClass),
		run.Summary, run.ErrorMessage, run.RetryCount,
		run.Acknowledged, run.AcknowledgedBy, tsOrNil(run.AcknowledgedAt),
		tsToTime(run.StartedAt), tsToTime(run.UpdatedAt), tsOrNil(run.FinishedAt), run.WorkflowName,
	).Exec(); err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	// Write to secondary index tables.
	srv.session.Query(`
		INSERT INTO workflow_runs_by_node (cluster_id, node_id, started_at, run_id, component_name, status, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ctx.ClusterId, ctx.NodeId, tsToTime(run.StartedAt), run.Id, ctx.ComponentName, int(run.Status), run.Summary,
	).Exec()

	srv.session.Query(`
		INSERT INTO workflow_runs_by_component (cluster_id, component_name, started_at, run_id, node_id, status, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ctx.ClusterId, ctx.ComponentName, tsToTime(run.StartedAt), run.Id, ctx.NodeId, int(run.Status), run.Summary,
	).Exec()

	logger.Info("workflow run started",
		"run_id", run.Id, "component", ctx.ComponentName,
		"node", ctx.NodeId, "trigger", run.TriggerReason.String())

	publishWorkflowEvent("workflow.run.started", map[string]interface{}{
		"run_id": run.Id, "component": ctx.ComponentName, "node_id": ctx.NodeId,
		"node_hostname": ctx.NodeHostname, "version": ctx.ComponentVersion,
		"status": run.Status.String(), "trigger": run.TriggerReason.String(),
	})

	return run, nil
}

func (srv *server) UpdateRun(_ context.Context, req *workflowpb.UpdateRunRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	now := time.Now()
	// ScyllaDB doesn't support subqueries in UPDATE WHERE. Use two-step approach.
	return nil, srv.updateRunByID(req.ClusterId, req.Id, func(startedAt time.Time) error {
		return srv.session.Query(`
			UPDATE workflow_runs SET status=?, summary=?, current_actor=?, updated_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			int(req.Status), req.Summary, int(req.CurrentActor), now,
			req.ClusterId, startedAt, req.Id,
		).Exec()
	})
}

func (srv *server) FinishRun(_ context.Context, req *workflowpb.FinishRunRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	now := time.Now()
	var (
		runStartedAt time.Time
		workflowName string
	)
	err := srv.updateRunByID(req.ClusterId, req.Id, func(startedAt time.Time) error {
		runStartedAt = startedAt

		// ── Protect terminal runs from stale overwrites ─────────────
		// A run that is already SUCCEEDED, SUPERSEDED, or CANCELLED must
		// never be overwritten by a stale callback. This prevents out-of-
		// order FinishRun calls from corrupting truth.
		var currentStatus int
		if scanErr := srv.session.Query(`
			SELECT status FROM workflow_runs
			WHERE cluster_id=? AND started_at=? AND id=? LIMIT 1`,
			req.ClusterId, startedAt, req.Id,
		).Scan(&currentStatus); scanErr == nil {
			cs := workflowpb.RunStatus(currentStatus)
			if isTerminalStatus(cs) && cs != req.Status {
				slog.Warn("FinishRun: refusing to overwrite terminal run",
					"run_id", req.Id,
					"current", cs.String(),
					"requested", req.Status.String())
				return nil
			}
		}

		// Load workflow_name to update the summary table after the run row is updated.
		_ = srv.session.Query(`
			SELECT workflow_name FROM workflow_runs
			WHERE cluster_id=? AND started_at=? AND id=? LIMIT 1`,
			req.ClusterId, startedAt, req.Id,
		).Scan(&workflowName)
		return srv.session.Query(`
			UPDATE workflow_runs SET status=?, failure_class=?, summary=?, error_message=?, updated_at=?, finished_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			int(req.Status), int(req.FailureClass), req.Summary, req.ErrorMessage, now, now,
			req.ClusterId, startedAt, req.Id,
		).Exec()
	})
	if err == nil {
		topic := "workflow.run.finished"
		if req.Status == workflowpb.RunStatus_RUN_STATUS_FAILED {
			topic = "workflow.run.failed"
		}
		publishWorkflowEvent(topic, map[string]interface{}{
			"run_id": req.Id, "status": req.Status.String(),
			"failure_class": req.FailureClass.String(), "summary": req.Summary,
			"error": req.ErrorMessage,
		})
		// Update bounded summary table so dashboards have O(# workflow types).
		failureReason := req.ErrorMessage
		if failureReason == "" {
			failureReason = req.Summary
		}
		durationMs := now.Sub(runStartedAt).Milliseconds()
		srv.upsertWorkflowSummary(req.ClusterId, workflowName, req.Id, req.Status,
			runStartedAt, now, durationMs, failureReason)
	}
	return &emptypb.Empty{}, err
}

func (srv *server) RecordStep(_ context.Context, req *workflowpb.RecordStepRequest) (*workflowpb.WorkflowStep, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	step := req.GetStep()
	if step == nil {
		return nil, fmt.Errorf("step is required")
	}
	now := time.Now()
	if step.CreatedAt == nil {
		step.CreatedAt = timestamppb.New(now)
	}

	if err := srv.session.Query(`
		INSERT INTO workflow_steps (
			cluster_id, run_id, seq, step_key, title,
			actor, phase, status, attempt, source_actor, target_actor,
			created_at, started_at, finished_at, duration_ms,
			message, error_code, error_message,
			retryable, operator_action_required, action_hint, details_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ClusterId, step.RunId, step.Seq, step.StepKey, step.Title,
		int(step.Actor), int(step.Phase), int(step.Status), step.Attempt, int(step.SourceActor), int(step.TargetActor),
		tsToTime(step.CreatedAt), tsOrNil(step.StartedAt), tsOrNil(step.FinishedAt), step.DurationMs,
		step.Message, step.ErrorCode, step.ErrorMessage,
		step.Retryable, step.OperatorActionRequired, step.ActionHint, step.DetailsJson,
	).Exec(); err != nil {
		return nil, fmt.Errorf("insert step: %w", err)
	}

	return step, nil
}

func (srv *server) UpdateStep(_ context.Context, req *workflowpb.UpdateStepRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	// Guard: never overwrite a terminal step status (SUCCEEDED/FAILED) with a
	// stale result. Out-of-order or retried callbacks must not corrupt truth.
	if srv.session != nil {
		var currentStatus int
		if err := srv.session.Query(`
			SELECT status FROM workflow_steps
			WHERE cluster_id=? AND run_id=? AND seq=? LIMIT 1`,
			req.ClusterId, req.RunId, req.Seq,
		).Scan(&currentStatus); err == nil {
			if isTerminalStepStatus(workflowpb.StepStatus(currentStatus)) {
				slog.Warn("UpdateStep: refusing to overwrite terminal step",
					"run_id", req.RunId, "seq", req.Seq,
					"current", workflowpb.StepStatus(currentStatus).String(),
					"requested", req.Status.String())
				return &emptypb.Empty{}, nil
			}
		}
	}
	if err := srv.session.Query(`
		UPDATE workflow_steps SET status=?, message=?, duration_ms=?, finished_at=?
		WHERE cluster_id=? AND run_id=? AND seq=?`,
		int(req.Status), req.Message, req.DurationMs, time.Now(),
		req.ClusterId, req.RunId, req.Seq,
	).Exec(); err != nil {
		return nil, fmt.Errorf("update step: %w", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) FailStep(_ context.Context, req *workflowpb.FailStepRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	// Guard: never overwrite a SUCCEEDED step with FAILED. A stale failure
	// from an earlier attempt or network retry must not corrupt truth.
	if srv.session != nil {
		var currentStatus int
		if err := srv.session.Query(`
			SELECT status FROM workflow_steps
			WHERE cluster_id=? AND run_id=? AND seq=? LIMIT 1`,
			req.ClusterId, req.RunId, req.Seq,
		).Scan(&currentStatus); err == nil {
			cs := workflowpb.StepStatus(currentStatus)
			if cs == workflowpb.StepStatus_STEP_STATUS_SUCCEEDED {
				slog.Warn("FailStep: refusing to downgrade SUCCEEDED step to FAILED",
					"run_id", req.RunId, "seq", req.Seq)
				return &emptypb.Empty{}, nil
			}
		}
	}
	now := time.Now()
	if err := srv.session.Query(`
		UPDATE workflow_steps SET status=?, error_code=?, error_message=?, action_hint=?,
		retryable=?, operator_action_required=?, finished_at=?
		WHERE cluster_id=? AND run_id=? AND seq=?`,
		int(workflowpb.StepStatus_STEP_STATUS_FAILED), req.ErrorCode, req.ErrorMessage, req.ActionHint,
		req.Retryable, req.OperatorActionRequired, now,
		req.ClusterId, req.RunId, req.Seq,
	).Exec(); err != nil {
		return nil, fmt.Errorf("fail step: %w", err)
	}

	publishWorkflowEvent("workflow.step.failed", map[string]interface{}{
		"run_id": req.RunId, "seq": req.Seq, "error_code": req.ErrorCode,
		"error": req.ErrorMessage, "hint": req.ActionHint,
		"retryable": req.Retryable, "failure_class": req.FailureClass.String(),
	})

	return &emptypb.Empty{}, nil
}

func (srv *server) AddArtifactRef(_ context.Context, req *workflowpb.AddArtifactRefRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	a := req.GetArtifact()
	if a == nil {
		return nil, fmt.Errorf("artifact is required")
	}
	if a.Id == "" {
		a.Id = gocql.TimeUUID().String()
	}
	now := time.Now()
	if a.CreatedAt == nil {
		a.CreatedAt = timestamppb.New(now)
	}

	if err := srv.session.Query(`
		INSERT INTO workflow_artifact_refs (
			cluster_id, run_id, id, step_seq, kind,
			name, version, digest, node_id,
			path, etcd_key, unit_name, config_path,
			package_name, package_version, spec_path, script_path,
			metadata_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ClusterId, a.RunId, a.Id, a.StepSeq, int(a.Kind),
		a.Name, a.Version, a.Digest, a.NodeId,
		a.Path, a.EtcdKey, a.UnitName, a.ConfigPath,
		a.PackageName, a.PackageVersion, a.SpecPath, a.ScriptPath,
		a.MetadataJson, tsToTime(a.CreatedAt),
	).Exec(); err != nil {
		return nil, fmt.Errorf("insert artifact ref: %w", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) AppendEvent(_ context.Context, req *workflowpb.AppendEventRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	ev := req.GetEvent()
	if ev == nil {
		return nil, fmt.Errorf("event is required")
	}
	if ev.EventId == "" {
		ev.EventId = gocql.TimeUUID().String()
	}
	now := time.Now()
	if ev.CreatedAt == nil {
		ev.CreatedAt = timestamppb.New(now)
	}

	if err := srv.session.Query(`
		INSERT INTO workflow_events (
			cluster_id, run_id, event_at, event_id, step_seq,
			event_type, actor, old_value, new_value, message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ClusterId, ev.RunId, tsToTime(ev.CreatedAt), ev.EventId, ev.StepSeq,
		ev.EventType, int(ev.Actor), ev.OldValue, ev.NewValue, ev.Message,
	).Exec(); err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	// Fan out to watchers.
	srv.fanoutEvent(ev)

	return &emptypb.Empty{}, nil
}

// ---------------------------------------------------------------------------
// Read RPCs
// ---------------------------------------------------------------------------

func (srv *server) GetRun(_ context.Context, req *workflowpb.GetRunRequest) (*workflowpb.WorkflowRunDetail, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	run, err := srv.loadRunByID(req.ClusterId, req.Id)
	if err != nil {
		return nil, err
	}

	steps, err := srv.loadSteps(req.ClusterId, req.Id)
	if err != nil {
		return nil, err
	}

	artifacts, err := srv.loadArtifacts(req.ClusterId, req.Id)
	if err != nil {
		return nil, err
	}

	return &workflowpb.WorkflowRunDetail{
		Run:       run,
		Steps:     steps,
		Artifacts: artifacts,
	}, nil
}

func (srv *server) ListRuns(_ context.Context, req *workflowpb.ListRunsRequest) (*workflowpb.ListRunsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	var runs []*workflowpb.WorkflowRun

	// Route to the appropriate index table based on filters.
	switch {
	case req.NodeId != "":
		runs = srv.listRunsByNode(req.ClusterId, req.NodeId, limit)
	case req.ComponentName != "":
		runs = srv.listRunsByComponent(req.ClusterId, req.ComponentName, limit)
	default:
		runs = srv.listRunsAll(req.ClusterId, limit)
	}

	// Apply in-memory filters for status/kind/active/failed.
	filtered := make([]*workflowpb.WorkflowRun, 0, len(runs))
	for _, r := range runs {
		if req.Status != workflowpb.RunStatus_RUN_STATUS_UNKNOWN && r.Status != req.Status {
			continue
		}
		if req.Kind != workflowpb.ComponentKind_COMPONENT_KIND_UNKNOWN && r.Context != nil && r.Context.ComponentKind != req.Kind {
			continue
		}
		if req.ActiveOnly && !isActiveStatus(r.Status) {
			continue
		}
		if req.FailedOnly && r.Status != workflowpb.RunStatus_RUN_STATUS_FAILED {
			continue
		}
		if req.WorkflowName != "" && r.WorkflowName != req.WorkflowName {
			continue
		}
		filtered = append(filtered, r)
	}

	return &workflowpb.ListRunsResponse{
		Runs:  filtered,
		Total: int32(len(filtered)),
	}, nil
}

func (srv *server) GetRunEvents(_ context.Context, req *workflowpb.GetRunEventsRequest) (*workflowpb.GetRunEventsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	iter := srv.session.Query(`
		SELECT run_id, event_id, step_seq, event_type, actor, old_value, new_value, message, event_at
		FROM workflow_events WHERE cluster_id=? AND run_id=?`,
		req.ClusterId, req.RunId,
	).Iter()

	var events []*workflowpb.WorkflowEvent
	var (
		runID, eventID, eventType, oldVal, newVal, msg string
		stepSeq, actor                                 int
		eventAt                                        time.Time
	)
	for iter.Scan(&runID, &eventID, &stepSeq, &eventType, &actor, &oldVal, &newVal, &msg, &eventAt) {
		events = append(events, &workflowpb.WorkflowEvent{
			RunId:     runID,
			EventId:   eventID,
			StepSeq:   int32(stepSeq),
			EventType: eventType,
			Actor:     workflowpb.WorkflowActor(actor),
			OldValue:  oldVal,
			NewValue:  newVal,
			Message:   msg,
			CreatedAt: timestamppb.New(eventAt),
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	return &workflowpb.GetRunEventsResponse{Events: events}, nil
}

func (srv *server) GetCurrentRunsForNode(ctx context.Context, req *workflowpb.GetCurrentRunsForNodeRequest) (*workflowpb.ListRunsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	return srv.ListRuns(ctx, &workflowpb.ListRunsRequest{
		ClusterId:  req.ClusterId,
		NodeId:     req.NodeId,
		ActiveOnly: true,
		Limit:      20,
	})
}

func (srv *server) GetComponentHistory(ctx context.Context, req *workflowpb.GetComponentHistoryRequest) (*workflowpb.ListRunsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	return srv.ListRuns(ctx, &workflowpb.ListRunsRequest{
		ClusterId:     req.ClusterId,
		ComponentName: req.ComponentName,
		Limit:         limit,
	})
}

func (srv *server) GetWorkflowGraph(_ context.Context, req *workflowpb.GetWorkflowGraphRequest) (*workflowpb.WorkflowGraph, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	detail, err := srv.GetRun(context.Background(), &workflowpb.GetRunRequest{
		ClusterId: req.ClusterId,
		Id:        req.RunId,
	})
	if err != nil {
		return nil, err
	}

	graph := &workflowpb.WorkflowGraph{
		Run:       detail.Run,
		Artifacts: detail.Artifacts,
	}

	// Group steps by phase.
	phaseMap := make(map[workflowpb.WorkflowPhaseKind]*workflowpb.WorkflowPhase)
	laneMap := make(map[workflowpb.WorkflowActor]*workflowpb.WorkflowActorLane)

	for _, step := range detail.Steps {
		// Phases
		p, ok := phaseMap[step.Phase]
		if !ok {
			p = &workflowpb.WorkflowPhase{
				Kind:        step.Phase,
				DisplayName: phaseDisplayName(step.Phase),
				Status:      workflowpb.StepStatus_STEP_STATUS_PENDING,
			}
			phaseMap[step.Phase] = p
		}
		p.Steps = append(p.Steps, step)
		if step.Status == workflowpb.StepStatus_STEP_STATUS_RUNNING {
			p.Status = workflowpb.StepStatus_STEP_STATUS_RUNNING
			graph.CurrentStepSeq = step.Seq
			graph.CurrentActor = step.Actor
		} else if step.Status == workflowpb.StepStatus_STEP_STATUS_FAILED {
			p.Status = workflowpb.StepStatus_STEP_STATUS_FAILED
		} else if step.Status == workflowpb.StepStatus_STEP_STATUS_SUCCEEDED && p.Status == workflowpb.StepStatus_STEP_STATUS_PENDING {
			p.Status = workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
		}

		// Actor lanes
		lane, ok := laneMap[step.Actor]
		if !ok {
			lane = &workflowpb.WorkflowActorLane{Actor: step.Actor}
			laneMap[step.Actor] = lane
		}
		lane.Steps = append(lane.Steps, step)
	}

	// Order phases by enum value.
	for pk := workflowpb.WorkflowPhaseKind(1); pk <= workflowpb.WorkflowPhaseKind_PHASE_COMPLETE; pk++ {
		if p, ok := phaseMap[pk]; ok {
			graph.Phases = append(graph.Phases, p)
		}
	}

	// Order actor lanes by enum value.
	for ak := workflowpb.WorkflowActor(1); ak <= workflowpb.WorkflowActor_ACTOR_AI_EXECUTOR; ak++ {
		if l, ok := laneMap[ak]; ok {
			graph.Lanes = append(graph.Lanes, l)
		}
	}

	if detail.Run != nil {
		graph.CurrentActor = detail.Run.CurrentActor
	}

	return graph, nil
}

// ---------------------------------------------------------------------------
// Streaming RPCs
// ---------------------------------------------------------------------------

func (srv *server) WatchRun(req *workflowpb.WatchRunRequest, stream workflowpb.WorkflowService_WatchRunServer) error {
	ch := make(chan *workflowpb.WorkflowEventEnvelope, 64)
	srv.addWatcher(req.RunId, ch)
	defer srv.removeWatcher(req.RunId, ch)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case env, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(env); err != nil {
				return err
			}
		}
	}
}

func (srv *server) WatchNodeRuns(req *workflowpb.WatchNodeRunsRequest, stream workflowpb.WorkflowService_WatchNodeRunsServer) error {
	ch := make(chan *workflowpb.WorkflowEventEnvelope, 64)
	srv.addNodeWatcher(req.NodeId, ch)
	defer srv.removeNodeWatcher(req.NodeId, ch)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case env, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(env); err != nil {
				return err
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Action RPCs (stubs for now)
// ---------------------------------------------------------------------------

func (srv *server) RetryRun(_ context.Context, req *workflowpb.RetryRunRequest) (*workflowpb.WorkflowRun, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	original, err := srv.loadRunByID(req.ClusterId, req.RunId)
	if err != nil {
		return nil, fmt.Errorf("load original run: %w", err)
	}
	if isActiveStatus(original.Status) {
		return nil, fmt.Errorf("cannot retry run %s: still active (status=%s)", req.RunId, original.Status)
	}

	// Create a new run linked to the original via ParentRunId,
	// preserving the same correlation_id so they appear in the same lineage.
	newRun := &workflowpb.WorkflowRun{
		Id:            gocql.TimeUUID().String(),
		CorrelationId: original.CorrelationId,
		ParentRunId:   original.Id,
		Context:       original.Context,
		TriggerReason: workflowpb.TriggerReason_TRIGGER_REASON_MANUAL,
		Status:        workflowpb.RunStatus_RUN_STATUS_PENDING,
		CurrentActor:  workflowpb.WorkflowActor_ACTOR_OPERATOR,
		RetryCount:    original.RetryCount + 1,
		StartedAt:     timestamppb.Now(),
		UpdatedAt:     timestamppb.Now(),
	}

	resp, err := srv.StartRun(context.Background(), &workflowpb.StartRunRequest{Run: newRun})
	if err != nil {
		return nil, fmt.Errorf("create retry run: %w", err)
	}

	logger.Info("workflow run retried", "original", req.RunId, "new", resp.Id, "component", original.Context.GetComponentName())
	return resp, nil
}

func (srv *server) CancelRun(_ context.Context, req *workflowpb.CancelRunRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	run, err := srv.loadRunByID(req.ClusterId, req.RunId)
	if err != nil {
		return nil, fmt.Errorf("load run: %w", err)
	}
	if !isActiveStatus(run.Status) {
		return nil, fmt.Errorf("cannot cancel run %s: already terminal (status=%s)", req.RunId, run.Status)
	}

	now := time.Now()
	if err := srv.updateRunByID(req.ClusterId, req.RunId, func(startedAt time.Time) error {
		return srv.session.Query(`
			UPDATE workflow_runs SET status=?, summary=?, error_message=?, finished_at=?, updated_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			int(workflowpb.RunStatus_RUN_STATUS_CANCELED),
			"Cancelled by operator",
			"operator cancelled",
			now, now,
			req.ClusterId, startedAt, req.RunId,
		).Exec()
	}); err != nil {
		return nil, fmt.Errorf("cancel run: %w", err)
	}

	logger.Info("workflow run cancelled", "run_id", req.RunId)
	return &emptypb.Empty{}, nil
}

func (srv *server) AcknowledgeRun(_ context.Context, req *workflowpb.AcknowledgeRunRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	now := time.Now()
	return &emptypb.Empty{}, srv.updateRunByID(req.ClusterId, req.RunId, func(startedAt time.Time) error {
		return srv.session.Query(`
			UPDATE workflow_runs SET acknowledged=?, acknowledged_by=?, acknowledged_at=?, updated_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			true, req.AcknowledgedBy, now, now,
			req.ClusterId, startedAt, req.RunId,
		).Exec()
	})
}

func (srv *server) DiagnoseRun(_ context.Context, req *workflowpb.DiagnoseRunRequest) (*workflowpb.DiagnoseRunResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	run, err := srv.loadRunByID(req.ClusterId, req.RunId)
	if err != nil {
		return nil, fmt.Errorf("load run: %w", err)
	}

	steps, _ := srv.loadSteps(req.ClusterId, req.RunId)
	artifacts, _ := srv.loadArtifacts(req.ClusterId, req.RunId)

	// Build diagnosis from run state.
	var diagnosis strings.Builder
	diagnosis.WriteString(fmt.Sprintf("Run %s for %s on node %s\n", run.Id, run.Context.GetComponentName(), run.Context.GetNodeId()))
	diagnosis.WriteString(fmt.Sprintf("Status: %s | Failure: %s\n", run.Status, run.FailureClass))
	if run.ErrorMessage != "" {
		diagnosis.WriteString(fmt.Sprintf("Error: %s\n", run.ErrorMessage))
	}

	// Identify the failed step.
	var failedStep *workflowpb.WorkflowStep
	for _, s := range steps {
		if s.Status == workflowpb.StepStatus_STEP_STATUS_FAILED {
			failedStep = s
			break
		}
	}
	if failedStep != nil {
		diagnosis.WriteString(fmt.Sprintf("\nFailed at step: %s (%s)\n", failedStep.StepKey, failedStep.Title))
		diagnosis.WriteString(fmt.Sprintf("Phase: %s | Actor: %s\n", phaseDisplayName(failedStep.Phase), failedStep.Actor))
		if failedStep.ErrorCode != "" {
			diagnosis.WriteString(fmt.Sprintf("Error code: %s\n", failedStep.ErrorCode))
		}
		if failedStep.ErrorMessage != "" {
			diagnosis.WriteString(fmt.Sprintf("Step error: %s\n", failedStep.ErrorMessage))
		}
		if failedStep.ActionHint != "" {
			diagnosis.WriteString(fmt.Sprintf("Hint: %s\n", failedStep.ActionHint))
		}
	}

	if len(artifacts) > 0 {
		diagnosis.WriteString(fmt.Sprintf("\nArtifacts involved: %d\n", len(artifacts)))
		for _, a := range artifacts {
			diagnosis.WriteString(fmt.Sprintf("  - %s %s (%s)\n", a.Kind, a.Name, a.Version))
		}
	}

	// Suggest action based on failure class.
	var suggestion string
	confidence := "medium"
	switch run.FailureClass {
	case workflowpb.FailureClass_FAILURE_CLASS_CONFIG:
		suggestion = "Check service configuration, release spec, and artifact digest. Verify the package was published correctly."
		confidence = "high"
	case workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY:
		suggestion = "Check that upstream dependencies are installed and healthy. The plan slot may be occupied by another plan."
		confidence = "high"
	case workflowpb.FailureClass_FAILURE_CLASS_SYSTEMD:
		suggestion = "Check systemd unit status and journal logs on the target node. The service may be crash-looping due to missing dependencies."
		confidence = "medium"
	case workflowpb.FailureClass_FAILURE_CLASS_NETWORK:
		suggestion = "Check network connectivity, TLS certificates, and DNS resolution between nodes."
		confidence = "medium"
	case workflowpb.FailureClass_FAILURE_CLASS_PACKAGE:
		suggestion = "Check package signing keys, dpkg configuration, and that the package version exists in the repository."
		confidence = "high"
	case workflowpb.FailureClass_FAILURE_CLASS_REPOSITORY:
		suggestion = "Verify the artifact exists in the repository with the expected checksum. Re-publish with --force if needed."
		confidence = "high"
	case workflowpb.FailureClass_FAILURE_CLASS_VALIDATION:
		suggestion = "The service started but readiness checks failed. Check service logs and port availability."
		confidence = "medium"
	default:
		suggestion = "Review the failed step error message and node agent logs for details."
		confidence = "low"
	}

	// Find related runs (same component, recent failures).
	var relatedIDs []string
	relatedRuns := srv.listRunsByComponent(req.ClusterId, run.Context.GetComponentName(), 10)
	for _, r := range relatedRuns {
		if r.Id != run.Id && r.Status == workflowpb.RunStatus_RUN_STATUS_FAILED {
			relatedIDs = append(relatedIDs, r.Id)
		}
	}

	return &workflowpb.DiagnoseRunResponse{
		Diagnosis:       diagnosis.String(),
		Confidence:      confidence,
		RelatedRunIds:   relatedIDs,
		SuggestedAction: suggestion,
	}, nil
}

// ---------------------------------------------------------------------------
// Workflow Definitions (MinIO-backed single source of truth)
// ---------------------------------------------------------------------------

// ListWorkflowDefinitions returns the list of YAML workflow definitions stored
// in MinIO under globular-config/workflows/.
func (srv *server) ListWorkflowDefinitions(_ context.Context, _ *workflowpb.ListWorkflowDefinitionsRequest) (*workflowpb.ListWorkflowDefinitionsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	keys, err := config.ListClusterConfigPrefix("workflows/")
	if err != nil {
		return nil, fmt.Errorf("list workflow definitions: %w", err)
	}

	var defs []*workflowpb.WorkflowDefinitionSummary
	for _, key := range keys {
		if !strings.HasSuffix(key, ".yaml") {
			continue
		}
		// Extract the workflow name from the key (e.g. "workflows/day0.bootstrap.yaml" → "day0.bootstrap")
		name := strings.TrimPrefix(key, "workflows/")
		name = strings.TrimSuffix(name, ".yaml")

		// Read the YAML to extract displayName and description
		data, err := config.GetClusterConfig(key)
		if err != nil || data == nil {
			defs = append(defs, &workflowpb.WorkflowDefinitionSummary{Name: name})
			continue
		}
		displayName, description := parseWorkflowMetadata(string(data), name)
		defs = append(defs, &workflowpb.WorkflowDefinitionSummary{
			Name:        name,
			DisplayName: displayName,
			Description: description,
		})
	}

	return &workflowpb.ListWorkflowDefinitionsResponse{Definitions: defs}, nil
}

// GetWorkflowDefinition returns the raw YAML content for a workflow definition.
func (srv *server) GetWorkflowDefinition(_ context.Context, req *workflowpb.GetWorkflowDefinitionRequest) (*workflowpb.GetWorkflowDefinitionResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}
	key := "workflows/" + req.Name + ".yaml"
	data, err := config.GetClusterConfig(key)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", key, err)
	}
	if data == nil {
		return nil, fmt.Errorf("workflow definition %q not found", req.Name)
	}
	return &workflowpb.GetWorkflowDefinitionResponse{
		Name:        req.Name,
		YamlContent: string(data),
	}, nil
}

// parseWorkflowMetadata extracts displayName and description from a workflow YAML
// without full parsing — uses line-by-line string scanning for simplicity.
func parseWorkflowMetadata(yamlContent, fallbackName string) (displayName, description string) {
	displayName = fallbackName
	inMetadata := false
	inDescriptionBlock := false
	var descLines []string

	for _, line := range strings.Split(yamlContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "metadata:") {
			inMetadata = true
			continue
		}
		if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			// Left the metadata block
			inMetadata = false
			inDescriptionBlock = false
		}
		if !inMetadata {
			continue
		}
		if strings.HasPrefix(trimmed, "displayName:") {
			displayName = strings.TrimSpace(strings.TrimPrefix(trimmed, "displayName:"))
			displayName = strings.Trim(displayName, `"'`)
			continue
		}
		if strings.HasPrefix(trimmed, "description:") {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			if rest == ">-" || rest == ">" || rest == "|" || rest == "|-" {
				inDescriptionBlock = true
				continue
			}
			description = strings.Trim(rest, `"'`)
			continue
		}
		if inDescriptionBlock {
			// Continue collecting description lines while indented deeper than "description:"
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t\t") {
				descLines = append(descLines, trimmed)
			} else {
				inDescriptionBlock = false
			}
		}
	}
	if description == "" && len(descLines) > 0 {
		description = strings.Join(descLines, " ")
	}
	return
}

// ---------------------------------------------------------------------------
// Workflow Run Summaries (bounded per-workflow-name aggregates)
// ---------------------------------------------------------------------------

// isTerminalRunStatus reports whether a RunStatus is terminal (no further
// transitions expected). Summary updates happen only on terminal transitions.
func isTerminalRunStatus(s workflowpb.RunStatus) bool {
	switch s {
	case workflowpb.RunStatus_RUN_STATUS_SUCCEEDED,
		workflowpb.RunStatus_RUN_STATUS_FAILED,
		workflowpb.RunStatus_RUN_STATUS_CANCELED,
		workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK,
		workflowpb.RunStatus_RUN_STATUS_SUPERSEDED:
		return true
	}
	return false
}

// upsertWorkflowSummary updates the workflow_run_summaries row for the given
// (cluster_id, workflow_name). Uses read-modify-write; concurrent writers for
// the same workflow_name may race, but this is acceptable for aggregate stats.
func (srv *server) upsertWorkflowSummary(clusterID, workflowName, runID string,
	status workflowpb.RunStatus, startedAt, finishedAt time.Time, durationMs int64, failureReason string) {
	if clusterID == "" || workflowName == "" {
		return
	}
	if !isTerminalRunStatus(status) {
		return
	}

	// Read existing summary (ignore error: missing row → zeroed fields).
	var (
		total, succ, fail                               int64
		lastSuccessID, lastFailureID, lastFailureReason string
		lastSuccessAt, lastFailureAt                    time.Time
	)
	_ = srv.session.Query(`
		SELECT total_runs, success_runs, failure_runs,
			last_success_id, last_success_at,
			last_failure_id, last_failure_at, last_failure_reason
		FROM workflow_run_summaries WHERE cluster_id=? AND workflow_name=?`,
		clusterID, workflowName,
	).Scan(&total, &succ, &fail, &lastSuccessID, &lastSuccessAt,
		&lastFailureID, &lastFailureAt, &lastFailureReason)

	total++
	if status == workflowpb.RunStatus_RUN_STATUS_SUCCEEDED {
		succ++
		lastSuccessID = runID
		lastSuccessAt = finishedAt
	} else {
		fail++
		lastFailureID = runID
		lastFailureAt = finishedAt
		if failureReason != "" {
			lastFailureReason = failureReason
		}
	}

	if err := srv.session.Query(`
		INSERT INTO workflow_run_summaries (
			cluster_id, workflow_name,
			total_runs, success_runs, failure_runs,
			last_run_id, last_run_status,
			last_started_at, last_finished_at, last_duration_ms,
			last_success_id, last_success_at,
			last_failure_id, last_failure_at, last_failure_reason,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		clusterID, workflowName,
		total, succ, fail,
		runID, int(status),
		startedAt, finishedAt, durationMs,
		lastSuccessID, tsTimeOrNil(lastSuccessAt),
		lastFailureID, tsTimeOrNil(lastFailureAt), lastFailureReason,
		time.Now(),
	).Exec(); err != nil {
		logger.Warn("upsert workflow summary failed", "cluster", clusterID, "workflow", workflowName, "err", err)
	}
}

// tsTimeOrNil returns nil for a zero time so Scylla stores NULL instead of epoch.
func tsTimeOrNil(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// RecordOutcome updates only the workflow summary table (no individual run row).
// Used by periodic workflows (e.g. cluster.reconcile firing every 30s) that
// would otherwise inflate storage with per-run detail.
func (srv *server) RecordOutcome(_ context.Context, req *workflowpb.RecordOutcomeRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil {
		return &emptypb.Empty{}, nil
	}
	srv.upsertWorkflowSummary(
		req.GetClusterId(), req.GetWorkflowName(), req.GetRunId(),
		req.GetStatus(),
		tsToTime(req.GetStartedAt()), tsToTime(req.GetFinishedAt()),
		req.GetDurationMs(), req.GetFailureReason(),
	)
	return &emptypb.Empty{}, nil
}

// ListWorkflowSummaries returns all summary rows for a cluster, optionally
// filtered by workflow_name.
func (srv *server) ListWorkflowSummaries(_ context.Context, req *workflowpb.ListWorkflowSummariesRequest) (*workflowpb.ListWorkflowSummariesResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}

	query := `SELECT cluster_id, workflow_name, total_runs, success_runs, failure_runs,
		last_run_id, last_run_status,
		last_started_at, last_finished_at, last_duration_ms,
		last_success_id, last_success_at,
		last_failure_id, last_failure_at, last_failure_reason,
		updated_at
		FROM workflow_run_summaries WHERE cluster_id=?`
	args := []interface{}{req.ClusterId}
	if req.WorkflowName != "" {
		query += ` AND workflow_name=?`
		args = append(args, req.WorkflowName)
	}

	iter := srv.session.Query(query, args...).Iter()
	defer iter.Close()

	var summaries []*workflowpb.WorkflowRunSummary
	for {
		var (
			clusterID, workflowName, lastRunID       string
			lastSuccessID, lastFailureID             string
			lastFailureReason                        string
			total, succ, fail, durationMs            int64
			lastStatus                               int
			lastStarted, lastFinished, lastSuccessAt time.Time
			lastFailureAt, updatedAt                 time.Time
		)
		if !iter.Scan(&clusterID, &workflowName, &total, &succ, &fail,
			&lastRunID, &lastStatus,
			&lastStarted, &lastFinished, &durationMs,
			&lastSuccessID, &lastSuccessAt,
			&lastFailureID, &lastFailureAt, &lastFailureReason,
			&updatedAt) {
			break
		}
		summaries = append(summaries, &workflowpb.WorkflowRunSummary{
			ClusterId:         clusterID,
			WorkflowName:      workflowName,
			TotalRuns:         total,
			SuccessRuns:       succ,
			FailureRuns:       fail,
			LastRunId:         lastRunID,
			LastRunStatus:     workflowpb.RunStatus(lastStatus),
			LastStartedAt:     maybeTimestamp(lastStarted),
			LastFinishedAt:    maybeTimestamp(lastFinished),
			LastDurationMs:    durationMs,
			LastSuccessId:     lastSuccessID,
			LastSuccessAt:     maybeTimestamp(lastSuccessAt),
			LastFailureId:     lastFailureID,
			LastFailureAt:     maybeTimestamp(lastFailureAt),
			LastFailureReason: lastFailureReason,
			UpdatedAt:         maybeTimestamp(updatedAt),
		})
	}
	return &workflowpb.ListWorkflowSummariesResponse{Summaries: summaries}, nil
}

// ---------------------------------------------------------------------------
// Convergence telemetry (AI-facing diagnostic signals)
// ---------------------------------------------------------------------------

// RecordStepOutcome upserts per-step aggregate counters. Called from the
// workflow engine's OnStepDone hook. Bounded cardinality: O(#workflows × #steps).
func (srv *server) RecordStepOutcome(_ context.Context, req *workflowpb.RecordStepOutcomeRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.WorkflowName == "" || req.StepId == "" {
		return &emptypb.Empty{}, nil
	}

	var (
		total, succ, fail, skip int64
		firstSeen               time.Time
	)
	_ = srv.session.Query(`
		SELECT total_executions, success_count, failure_count, skipped_count, first_seen_at
		FROM workflow_step_outcomes WHERE cluster_id=? AND workflow_name=? AND step_id=?`,
		req.ClusterId, req.WorkflowName, req.StepId,
	).Scan(&total, &succ, &fail, &skip, &firstSeen)

	total++
	switch req.Status {
	case workflowpb.StepStatus_STEP_STATUS_SUCCEEDED:
		succ++
	case workflowpb.StepStatus_STEP_STATUS_FAILED:
		fail++
	case workflowpb.StepStatus_STEP_STATUS_SKIPPED:
		skip++
	}
	if firstSeen.IsZero() {
		firstSeen = time.Now()
	}

	if err := srv.session.Query(`
		INSERT INTO workflow_step_outcomes (
			cluster_id, workflow_name, step_id,
			total_executions, success_count, failure_count, skipped_count,
			last_status, last_started_at, last_finished_at, last_duration_ms,
			last_error_code, last_error_message,
			first_seen_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ClusterId, req.WorkflowName, req.StepId,
		total, succ, fail, skip,
		int(req.Status),
		tsTimeOrNil(tsToTime(req.StartedAt)),
		tsTimeOrNil(tsToTime(req.FinishedAt)),
		req.DurationMs,
		req.ErrorCode, req.ErrorMessage,
		firstSeen, time.Now(),
	).Exec(); err != nil {
		logger.Warn("record step outcome failed", "workflow", req.WorkflowName, "step", req.StepId, "err", err)
	}
	return &emptypb.Empty{}, nil
}

// ListStepOutcomes returns per-step aggregate counters, optionally filtered
// by workflow_name.
func (srv *server) ListStepOutcomes(_ context.Context, req *workflowpb.ListStepOutcomesRequest) (*workflowpb.ListStepOutcomesResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	query := `SELECT cluster_id, workflow_name, step_id,
		total_executions, success_count, failure_count, skipped_count,
		last_status, last_started_at, last_finished_at, last_duration_ms,
		last_error_code, last_error_message, first_seen_at, updated_at
		FROM workflow_step_outcomes WHERE cluster_id=?`
	args := []interface{}{req.ClusterId}
	if req.WorkflowName != "" {
		query += ` AND workflow_name=?`
		args = append(args, req.WorkflowName)
	}
	iter := srv.session.Query(query, args...).Iter()
	defer iter.Close()

	var out []*workflowpb.WorkflowStepOutcome
	for {
		var (
			clusterID, wfName, stepID    string
			errCode, errMsg              string
			total, succ, fail, skip, dur int64
			lastStatus                   int
			lastStarted, lastFinished    time.Time
			firstSeen, updatedAt         time.Time
		)
		if !iter.Scan(&clusterID, &wfName, &stepID,
			&total, &succ, &fail, &skip,
			&lastStatus, &lastStarted, &lastFinished, &dur,
			&errCode, &errMsg, &firstSeen, &updatedAt) {
			break
		}
		out = append(out, &workflowpb.WorkflowStepOutcome{
			ClusterId:        clusterID,
			WorkflowName:     wfName,
			StepId:           stepID,
			TotalExecutions:  total,
			SuccessCount:     succ,
			FailureCount:     fail,
			SkippedCount:     skip,
			LastStatus:       workflowpb.StepStatus(lastStatus),
			LastStartedAt:    maybeTimestamp(lastStarted),
			LastFinishedAt:   maybeTimestamp(lastFinished),
			LastDurationMs:   dur,
			LastErrorCode:    errCode,
			LastErrorMessage: errMsg,
			FirstSeenAt:      maybeTimestamp(firstSeen),
			UpdatedAt:        maybeTimestamp(updatedAt),
		})
	}
	return &workflowpb.ListStepOutcomesResponse{Outcomes: out}, nil
}

// RecordPhaseTransition appends a phase transition event (TTL 7 days).
// Called whenever a resource's phase changes or an invariant guard rejects a transition.
func (srv *server) RecordPhaseTransition(_ context.Context, req *workflowpb.RecordPhaseTransitionRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.ResourceType == "" || req.ResourceName == "" {
		return &emptypb.Empty{}, nil
	}
	now := time.Now()
	eventID := uuid.NewString()
	if err := srv.session.Query(`
		INSERT INTO phase_transition_log (
			cluster_id, resource_type, resource_name, event_at, event_id,
			from_phase, to_phase, reason, caller, blocked
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ClusterId, req.ResourceType, req.ResourceName, now, eventID,
		req.FromPhase, req.ToPhase, req.Reason, req.Caller, req.Blocked,
	).Exec(); err != nil {
		logger.Warn("record phase transition failed", "resource", req.ResourceName, "err", err)
	}
	return &emptypb.Empty{}, nil
}

// ListPhaseTransitions returns the transition history for a single resource.
func (srv *server) ListPhaseTransitions(_ context.Context, req *workflowpb.ListPhaseTransitionsRequest) (*workflowpb.ListPhaseTransitionsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.ResourceType == "" || req.ResourceName == "" {
		return nil, fmt.Errorf("cluster_id, resource_type, resource_name are required")
	}
	limit := int(req.Limit)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	iter := srv.session.Query(`
		SELECT event_at, event_id, from_phase, to_phase, reason, caller, blocked
		FROM phase_transition_log
		WHERE cluster_id=? AND resource_type=? AND resource_name=?
		LIMIT ?`,
		req.ClusterId, req.ResourceType, req.ResourceName, limit,
	).Iter()
	defer iter.Close()

	var events []*workflowpb.PhaseTransitionEvent
	for {
		var (
			eventAt                           time.Time
			eventID, from, to, reason, caller string
			blocked                           bool
		)
		if !iter.Scan(&eventAt, &eventID, &from, &to, &reason, &caller, &blocked) {
			break
		}
		events = append(events, &workflowpb.PhaseTransitionEvent{
			ClusterId:    req.ClusterId,
			ResourceType: req.ResourceType,
			ResourceName: req.ResourceName,
			EventAt:      maybeTimestamp(eventAt),
			EventId:      eventID,
			FromPhase:    from,
			ToPhase:      to,
			Reason:       reason,
			Caller:       caller,
			Blocked:      blocked,
		})
	}
	return &workflowpb.ListPhaseTransitionsResponse{Events: events}, nil
}

// RecordDriftObservation increments consecutive_cycles for a drift item or
// inserts it as a new observation. Callers should invoke this once per
// reconcile cycle for each drift item still present.
func (srv *server) RecordDriftObservation(_ context.Context, req *workflowpb.RecordDriftObservationRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.DriftType == "" || req.EntityRef == "" {
		return &emptypb.Empty{}, nil
	}

	var (
		cycles        int
		firstObserved time.Time
	)
	_ = srv.session.Query(`
		SELECT consecutive_cycles, first_observed_at
		FROM drift_unresolved WHERE cluster_id=? AND drift_type=? AND entity_ref=?`,
		req.ClusterId, req.DriftType, req.EntityRef,
	).Scan(&cycles, &firstObserved)

	cycles++
	now := time.Now()
	if firstObserved.IsZero() {
		firstObserved = now
	}

	if err := srv.session.Query(`
		INSERT INTO drift_unresolved (
			cluster_id, drift_type, entity_ref,
			consecutive_cycles, first_observed_at, last_observed_at,
			chosen_workflow, last_remediation_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ClusterId, req.DriftType, req.EntityRef,
		cycles, firstObserved, now, req.ChosenWorkflow, req.RemediationId,
	).Exec(); err != nil {
		logger.Warn("record drift observation failed", "drift_type", req.DriftType, "entity", req.EntityRef, "err", err)
	}
	return &emptypb.Empty{}, nil
}

// ClearDriftObservation removes a drift item when it's no longer observed.
func (srv *server) ClearDriftObservation(_ context.Context, req *workflowpb.ClearDriftObservationRequest) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.DriftType == "" || req.EntityRef == "" {
		return &emptypb.Empty{}, nil
	}
	if err := srv.session.Query(`
		DELETE FROM drift_unresolved WHERE cluster_id=? AND drift_type=? AND entity_ref=?`,
		req.ClusterId, req.DriftType, req.EntityRef,
	).Exec(); err != nil {
		logger.Warn("clear drift observation failed", "drift_type", req.DriftType, "entity", req.EntityRef, "err", err)
	}
	return &emptypb.Empty{}, nil
}

// ListDriftUnresolved returns drift items that have been observed >= min_cycles
// consecutive reconcile cycles without being cleared.
func (srv *server) ListDriftUnresolved(_ context.Context, req *workflowpb.ListDriftUnresolvedRequest) (*workflowpb.ListDriftUnresolvedResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	query := `SELECT cluster_id, drift_type, entity_ref, consecutive_cycles,
		first_observed_at, last_observed_at, chosen_workflow, last_remediation_id
		FROM drift_unresolved WHERE cluster_id=?`
	args := []interface{}{req.ClusterId}
	if req.DriftType != "" {
		query += ` AND drift_type=?`
		args = append(args, req.DriftType)
	}
	iter := srv.session.Query(query, args...).Iter()
	defer iter.Close()

	minCycles := int(req.MinCycles)
	var out []*workflowpb.DriftUnresolved
	for {
		var (
			cID, dType, eRef, chosen, remID string
			cycles                          int
			first, last                     time.Time
		)
		if !iter.Scan(&cID, &dType, &eRef, &cycles, &first, &last, &chosen, &remID) {
			break
		}
		if cycles < minCycles {
			continue
		}
		out = append(out, &workflowpb.DriftUnresolved{
			ClusterId:         cID,
			DriftType:         dType,
			EntityRef:         eRef,
			ConsecutiveCycles: int32(cycles),
			FirstObservedAt:   maybeTimestamp(first),
			LastObservedAt:    maybeTimestamp(last),
			ChosenWorkflow:    chosen,
			LastRemediationId: remID,
		})
	}
	return &workflowpb.ListDriftUnresolvedResponse{Items: out}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (srv *server) updateRunByID(clusterID, runID string, fn func(startedAt time.Time) error) error {
	var startedAt time.Time
	if err := srv.session.Query(`
		SELECT started_at FROM workflow_runs WHERE cluster_id=? AND id=? LIMIT 1 ALLOW FILTERING`,
		clusterID, runID,
	).Scan(&startedAt); err != nil {
		return fmt.Errorf("lookup run started_at: %w", err)
	}
	return fn(startedAt)
}

func (srv *server) loadRunByID(clusterID, runID string) (*workflowpb.WorkflowRun, error) {
	var (
		id, corrID, parentID                                        string
		nodeID, nodeHostname, compName, compVersion                 string
		relKind, relObjID, desObjID                                 string
		summary, errMsg, ackBy, wfName                              string
		compKind, trigReason, status, curActor, failClass, retryCnt int
		ack                                                         bool
		startedAt, updatedAt, finishedAt, ackAt                     time.Time
	)

	if err := srv.session.Query(`
		SELECT id, correlation_id, parent_run_id,
			node_id, node_hostname, component_name, component_kind, component_version,
			release_kind, release_object_id, desired_object_id,
			trigger_reason, status, current_actor, failure_class,
			summary, error_message, retry_count,
			acknowledged, acknowledged_by, acknowledged_at,
			started_at, updated_at, finished_at, workflow_name
		FROM workflow_runs WHERE cluster_id=? AND id=? LIMIT 1 ALLOW FILTERING`,
		clusterID, runID,
	).Scan(
		&id, &corrID, &parentID,
		&nodeID, &nodeHostname, &compName, &compKind, &compVersion,
		&relKind, &relObjID, &desObjID,
		&trigReason, &status, &curActor, &failClass,
		&summary, &errMsg, &retryCnt,
		&ack, &ackBy, &ackAt,
		&startedAt, &updatedAt, &finishedAt, &wfName,
	); err != nil {
		return nil, fmt.Errorf("load run %s: %w", runID, err)
	}

	return &workflowpb.WorkflowRun{
		Id:            id,
		CorrelationId: corrID,
		ParentRunId:   parentID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:        clusterID,
			NodeId:           nodeID,
			NodeHostname:     nodeHostname,
			ComponentName:    compName,
			ComponentKind:    workflowpb.ComponentKind(compKind),
			ComponentVersion: compVersion,
			ReleaseKind:      relKind,
			ReleaseObjectId:  relObjID,
			DesiredObjectId:  desObjID,
		},
		TriggerReason:  workflowpb.TriggerReason(trigReason),
		Status:         workflowpb.RunStatus(status),
		CurrentActor:   workflowpb.WorkflowActor(curActor),
		FailureClass:   workflowpb.FailureClass(failClass),
		Summary:        summary,
		ErrorMessage:   errMsg,
		RetryCount:     int32(retryCnt),
		Acknowledged:   ack,
		AcknowledgedBy: ackBy,
		AcknowledgedAt: maybeTimestamp(ackAt),
		StartedAt:      timestamppb.New(startedAt),
		UpdatedAt:      timestamppb.New(updatedAt),
		FinishedAt:     maybeTimestamp(finishedAt),
		WorkflowName:   wfName,
	}, nil
}

func (srv *server) loadSteps(clusterID, runID string) ([]*workflowpb.WorkflowStep, error) {
	iter := srv.session.Query(`
		SELECT run_id, seq, step_key, title, actor, phase, status, attempt,
			source_actor, target_actor, created_at, started_at, finished_at, duration_ms,
			message, error_code, error_message, retryable, operator_action_required, action_hint, details_json
		FROM workflow_steps WHERE cluster_id=? AND run_id=?`,
		clusterID, runID,
	).Iter()

	var steps []*workflowpb.WorkflowStep
	var (
		rID, stepKey, title, msg, errCode, errMsg, actionHint, detailsJSON string
		seq, actor, phase, status, attempt, srcActor, tgtActor             int
		retryable, opAction                                                bool
		createdAt, startedAt, finishedAt                                   time.Time
		durationMs                                                         int64
	)
	for iter.Scan(&rID, &seq, &stepKey, &title, &actor, &phase, &status, &attempt,
		&srcActor, &tgtActor, &createdAt, &startedAt, &finishedAt, &durationMs,
		&msg, &errCode, &errMsg, &retryable, &opAction, &actionHint, &detailsJSON) {
		steps = append(steps, &workflowpb.WorkflowStep{
			RunId: rID, Seq: int32(seq), StepKey: stepKey, Title: title,
			Actor: workflowpb.WorkflowActor(actor), Phase: workflowpb.WorkflowPhaseKind(phase),
			Status: workflowpb.StepStatus(status), Attempt: int32(attempt),
			SourceActor: workflowpb.WorkflowActor(srcActor), TargetActor: workflowpb.WorkflowActor(tgtActor),
			CreatedAt: timestamppb.New(createdAt), StartedAt: maybeTimestamp(startedAt), FinishedAt: maybeTimestamp(finishedAt),
			DurationMs: durationMs, Message: msg, ErrorCode: errCode, ErrorMessage: errMsg,
			Retryable: retryable, OperatorActionRequired: opAction, ActionHint: actionHint, DetailsJson: detailsJSON,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("query steps: %w", err)
	}
	return steps, nil
}

func (srv *server) loadArtifacts(clusterID, runID string) ([]*workflowpb.WorkflowArtifactRef, error) {
	iter := srv.session.Query(`
		SELECT id, run_id, step_seq, kind, name, version, digest, node_id,
			path, etcd_key, unit_name, config_path, package_name, package_version, spec_path, script_path,
			metadata_json, created_at
		FROM workflow_artifact_refs WHERE cluster_id=? AND run_id=?`,
		clusterID, runID,
	).Iter()

	var artifacts []*workflowpb.WorkflowArtifactRef
	var (
		aID, rID, name, ver, digest, nodeID                                     string
		path, etcdKey, unitName, cfgPath, pkgName, pkgVer, specPath, scriptPath string
		metaJSON                                                                string
		stepSeq, kind                                                           int
		createdAt                                                               time.Time
	)
	for iter.Scan(&aID, &rID, &stepSeq, &kind, &name, &ver, &digest, &nodeID,
		&path, &etcdKey, &unitName, &cfgPath, &pkgName, &pkgVer, &specPath, &scriptPath,
		&metaJSON, &createdAt) {
		artifacts = append(artifacts, &workflowpb.WorkflowArtifactRef{
			Id: aID, RunId: rID, StepSeq: int32(stepSeq), Kind: workflowpb.ArtifactKind(kind),
			Name: name, Version: ver, Digest: digest, NodeId: nodeID,
			Path: path, EtcdKey: etcdKey, UnitName: unitName, ConfigPath: cfgPath,
			PackageName: pkgName, PackageVersion: pkgVer, SpecPath: specPath, ScriptPath: scriptPath,
			MetadataJson: metaJSON, CreatedAt: timestamppb.New(createdAt),
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("query artifacts: %w", err)
	}
	return artifacts, nil
}

func (srv *server) listRunsAll(clusterID string, limit int) []*workflowpb.WorkflowRun {
	return srv.scanRunsSummary(`SELECT id, started_at FROM workflow_runs WHERE cluster_id=? LIMIT ?`, clusterID, limit)
}

func (srv *server) listRunsByNode(clusterID, nodeID string, limit int) []*workflowpb.WorkflowRun {
	var runs []*workflowpb.WorkflowRun
	iter := srv.session.Query(`
		SELECT run_id FROM workflow_runs_by_node WHERE cluster_id=? AND node_id=? LIMIT ?`,
		clusterID, nodeID, limit,
	).Iter()
	var runID string
	for iter.Scan(&runID) {
		if r, err := srv.loadRunByID(clusterID, runID); err == nil {
			runs = append(runs, r)
		}
	}
	iter.Close()
	return runs
}

func (srv *server) listRunsByComponent(clusterID, component string, limit int) []*workflowpb.WorkflowRun {
	var runs []*workflowpb.WorkflowRun
	iter := srv.session.Query(`
		SELECT run_id FROM workflow_runs_by_component WHERE cluster_id=? AND component_name=? LIMIT ?`,
		clusterID, component, limit,
	).Iter()
	var runID string
	for iter.Scan(&runID) {
		if r, err := srv.loadRunByID(clusterID, runID); err == nil {
			runs = append(runs, r)
		}
	}
	iter.Close()
	return runs
}

func (srv *server) scanRunsSummary(query string, args ...interface{}) []*workflowpb.WorkflowRun {
	iter := srv.session.Query(query, args...).Iter()
	var runs []*workflowpb.WorkflowRun
	var id string
	var startedAt time.Time
	for iter.Scan(&id, &startedAt) {
		if r, err := srv.loadRunByID(args[0].(string), id); err == nil {
			runs = append(runs, r)
		}
	}
	iter.Close()
	return runs
}

// ---------------------------------------------------------------------------
// Watcher management
// ---------------------------------------------------------------------------

func (srv *server) addWatcher(runID string, ch chan *workflowpb.WorkflowEventEnvelope) {
	srv.watchersMu.Lock()
	defer srv.watchersMu.Unlock()
	if srv.watchers == nil {
		srv.watchers = make(map[string][]chan *workflowpb.WorkflowEventEnvelope)
	}
	srv.watchers[runID] = append(srv.watchers[runID], ch)
}

func (srv *server) removeWatcher(runID string, ch chan *workflowpb.WorkflowEventEnvelope) {
	srv.watchersMu.Lock()
	defer srv.watchersMu.Unlock()
	chans := srv.watchers[runID]
	for i, c := range chans {
		if c == ch {
			srv.watchers[runID] = append(chans[:i], chans[i+1:]...)
			break
		}
	}
}

func (srv *server) addNodeWatcher(nodeID string, ch chan *workflowpb.WorkflowEventEnvelope) {
	srv.watchersMu.Lock()
	defer srv.watchersMu.Unlock()
	if srv.nodeWatch == nil {
		srv.nodeWatch = make(map[string][]chan *workflowpb.WorkflowEventEnvelope)
	}
	srv.nodeWatch[nodeID] = append(srv.nodeWatch[nodeID], ch)
}

func (srv *server) removeNodeWatcher(nodeID string, ch chan *workflowpb.WorkflowEventEnvelope) {
	srv.watchersMu.Lock()
	defer srv.watchersMu.Unlock()
	chans := srv.nodeWatch[nodeID]
	for i, c := range chans {
		if c == ch {
			srv.nodeWatch[nodeID] = append(chans[:i], chans[i+1:]...)
			break
		}
	}
}

func (srv *server) fanoutEvent(ev *workflowpb.WorkflowEvent) {
	srv.watchersMu.RLock()
	defer srv.watchersMu.RUnlock()

	env := &workflowpb.WorkflowEventEnvelope{
		RunId: ev.RunId,
		Event: ev,
	}

	// Fan out to run-specific watchers.
	for _, ch := range srv.watchers[ev.RunId] {
		select {
		case ch <- env:
		default: // drop if slow
		}
	}
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// publishWorkflowEvent emits a workflow event to the Event service.
// Fire-and-forget — never blocks the RPC pipeline.
func publishWorkflowEvent(topic string, payload map[string]interface{}) {
	go func() {
		globular.PublishEvent(topic, payload)
	}()
}

func tsToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func tsOrNil(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

func maybeTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func isActiveStatus(s workflowpb.RunStatus) bool {
	switch s {
	case workflowpb.RunStatus_RUN_STATUS_PENDING,
		workflowpb.RunStatus_RUN_STATUS_EXECUTING,
		workflowpb.RunStatus_RUN_STATUS_BLOCKED,
		workflowpb.RunStatus_RUN_STATUS_RETRYING:
		return true
	}
	return false
}

func phaseDisplayName(p workflowpb.WorkflowPhaseKind) string {
	switch p {
	case workflowpb.WorkflowPhaseKind_PHASE_DECISION:
		return "Decision"
	case workflowpb.WorkflowPhaseKind_PHASE_FETCH:
		return "Fetch"
	case workflowpb.WorkflowPhaseKind_PHASE_INSTALL:
		return "Install"
	case workflowpb.WorkflowPhaseKind_PHASE_CONFIGURE:
		return "Configure"
	case workflowpb.WorkflowPhaseKind_PHASE_START:
		return "Start"
	case workflowpb.WorkflowPhaseKind_PHASE_VERIFY:
		return "Verify"
	case workflowpb.WorkflowPhaseKind_PHASE_PUBLISH:
		return "Publish State"
	case workflowpb.WorkflowPhaseKind_PHASE_COMPLETE:
		return "Complete"
	}
	return "Unknown"
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

func initializeServerDefaults() *server {
	return &server{
		Name:                    "workflow.WorkflowService",
		Proto:                   "workflow.proto",
		Path:                    func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:                    defaultPort,
		Proxy:                   defaultProxy,
		Protocol:                "grpc",
		Version:                 Version,
		PublisherID:             "localhost",
		Description:             "Workflow service — reconciliation workflow tracing and history",
		Keywords:                []string{"workflow", "reconciliation", "trace", "operations", "scylladb"},
		AllowAllOrigins:         true,
		KeepAlive:               true,
		KeepUpToDate:            true,
		Process:                 -1,
		ProxyProcess:            -1,
		Repositories:            make([]string, 0),
		Discoveries:             make([]string, 0),
		Dependencies:            []string{},
		Permissions:             make([]interface{}, 0),
		ScyllaHosts:             nil, // resolved from etcd at Init() — never hardcode
		ScyllaPort:              9042,
		ScyllaReplicationFactor: 1,
	}
}

// scyllaHostsOrRoutable resolves ScyllaDB hosts from etcd, falling back to the
// node's routable IP so we never hard-code 127.0.0.1.
func scyllaHostsOrRoutable() []string {
	if hosts, err := config.GetScyllaHosts(); err == nil && len(hosts) > 0 {
		return hosts
	}
	if ip := config.GetRoutableIPv4(); ip != "" {
		return []string{ip}
	}
	return nil
}

func setupGrpcService(s *server) {
	workflowpb.RegisterWorkflowServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
}

func main() {
	// Enable etcd as the primary source for core workflow definitions.
	v1alpha1.EnableEtcdFetcher()

	srv := initializeServerDefaults()

	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = func() {
		exe := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stdout, "%s — Reconciliation Workflow Trace Service\n\nUsage: %s [flags] [domain [port]]\n\nFlags:\n", exe, exe)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	if *showHelp {
		flag.Usage()
		return
	}
	if *showVersion {
		data, _ := json.MarshalIndent(map[string]string{
			"service": srv.Name, "version": srv.Version, "build": BuildTime, "commit": GitCommit,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}
	if *showDescribe {
		globular.HandleDescribeFlag(srv, logger)
		return
	}
	if *showHealth {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"service": srv.Name, "status": "healthy", "version": srv.Version,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}

	args := flag.Args()
	// Workflow is a well-known service with fixed port (10220) — skip dynamic
	// allocation so controller and node-agent recorders can connect by convention.
	// Same pattern as DNS (fixed port 10006).
	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	logger.Info("starting workflow service", "service", srv.Name, "version", srv.Version, "domain", srv.Domain)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	setupGrpcService(srv)

	logger.Info("service ready",
		"service", srv.Name, "version", srv.Version,
		"port", srv.Port, "domain", srv.Domain,
		"startup_ms", time.Since(start).Milliseconds(),
	)

	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

// loadCAPool returns the cluster CA certificate pool for TLS connections.
func (srv *server) loadCAPool() *x509.CertPool {
	caFile := config.GetLocalCACertificate()
	if caFile == "" {
		return nil
	}
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return nil
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)
	return pool
}
