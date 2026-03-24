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

	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/workflow/workflowpb"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/gocql/gocql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	Version   = "0.0.1"
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
}

// ---------------------------------------------------------------------------
// Globular service contract
// ---------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string               { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)           { srv.ConfigPath = path }
func (srv *server) GetAddress() string                         { return srv.Address }
func (srv *server) SetAddress(address string)                  { srv.Address = address }
func (srv *server) GetProcess() int                            { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		srv.closeScylla()
	}
	srv.Process = pid
}
func (srv *server) GetProxyProcess() int                       { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)                    { srv.ProxyProcess = pid }
func (srv *server) GetState() string                           { return srv.State }
func (srv *server) SetState(state string)                      { srv.State = state }
func (srv *server) GetLastError() string                       { return srv.LastError }
func (srv *server) SetLastError(err string)                    { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)                   { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                          { return srv.ModTime }
func (srv *server) GetId() string                              { return srv.Id }
func (srv *server) SetId(id string)                            { srv.Id = id }
func (srv *server) GetName() string                            { return srv.Name }
func (srv *server) SetName(name string)                        { srv.Name = name }
func (srv *server) GetMac() string                             { return srv.Mac }
func (srv *server) SetMac(mac string)                          { srv.Mac = mac }
func (srv *server) GetDescription() string                     { return srv.Description }
func (srv *server) SetDescription(description string)          { srv.Description = description }
func (srv *server) GetKeywords() []string                      { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)              { srv.Keywords = keywords }
func (srv *server) Dist(path string) (string, error)           { return globular.Dist(path, srv) }
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
func (srv *server) GetChecksum() string                         { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)                 { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                         { return srv.Plaform }
func (srv *server) SetPlatform(platform string)                 { srv.Plaform = platform }
func (srv *server) GetRepositories() []string                   { return srv.Repositories }
func (srv *server) SetRepositories(v []string)                  { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string                    { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)                   { srv.Discoveries = v }
func (srv *server) GetPath() string                             { return srv.Path }
func (srv *server) SetPath(path string)                         { srv.Path = path }
func (srv *server) GetProto() string                            { return srv.Proto }
func (srv *server) SetProto(proto string)                       { srv.Proto = proto }
func (srv *server) GetPort() int                                { return srv.Port }
func (srv *server) SetPort(port int)                            { srv.Port = port }
func (srv *server) GetProxy() int                               { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                          { srv.Proxy = proxy }
func (srv *server) GetProtocol() string                         { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)                 { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                    { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)                   { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string                   { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)                  { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string                           { return srv.Domain }
func (srv *server) SetDomain(domain string)                     { srv.Domain = domain }
func (srv *server) GetTls() bool                                { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                          { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string               { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)             { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                         { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)                 { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                          { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)                   { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                          { return srv.Version }
func (srv *server) SetVersion(version string)                   { srv.Version = version }
func (srv *server) GetPublisherID() string                      { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)                     { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool                       { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                    { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                          { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                       { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}               { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{})    { srv.Permissions = permissions }
func (srv *server) GetGrpcServer() *grpc.Server                 { return srv.grpcServer }
func (srv *server) Save() error                                 { return globular.SaveService(srv) }
func (srv *server) StartService() error                         { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error                          { return globular.StopService(srv, srv.grpcServer) }
func (srv *server) RolesDefault() []resourcepb.Role             { return []resourcepb.Role{} }

// ---------------------------------------------------------------------------
// ScyllaDB connection
// ---------------------------------------------------------------------------

func (srv *server) connectScylla() error {
	if srv.session != nil {
		return nil
	}

	hosts := srv.ScyllaHosts
	if len(hosts) == 0 {
		hosts = []string{"127.0.0.1"}
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
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect: %w", err)
	}

	cql := fmt.Sprintf(createWorkflowKeyspaceCQL, rf)
	if err := session.Query(cql).Exec(); err != nil {
		session.Close()
		return fmt.Errorf("create keyspace: %w", err)
	}

	for _, stmt := range schemaCQLStatements {
		if err := session.Query(stmt).Exec(); err != nil {
			session.Close()
			return fmt.Errorf("schema init: %w", err)
		}
	}
	session.Close()

	cluster.Keyspace = workflowKeyspace
	srv.session, err = cluster.CreateSession()
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

	if h := os.Getenv("SCYLLA_HOSTS"); h != "" {
		srv.ScyllaHosts = strings.Split(h, ",")
	}
	if p := os.Getenv("SCYLLA_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &srv.ScyllaPort)
	}

	// ScyllaDB binds to the routable IP, not 127.0.0.1. If still on default,
	// fall back to this service's advertised address (which is the node IP).
	if len(srv.ScyllaHosts) == 0 || (len(srv.ScyllaHosts) == 1 && srv.ScyllaHosts[0] == "127.0.0.1") {
		if srv.Address != "" {
			host := srv.Address
			if h, _, ok := strings.Cut(host, ":"); ok && h != "" {
				host = h
			}
			if host != "" && host != "127.0.0.1" && host != "localhost" {
				srv.ScyllaHosts = []string{host}
			}
		}
	}

	if err := srv.connectScylla(); err != nil {
		return fmt.Errorf("scylla init: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Write RPCs
// ---------------------------------------------------------------------------

func (srv *server) StartRun(_ context.Context, req *workflowpb.StartRunRequest) (*workflowpb.WorkflowRun, error) {
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

	if err := srv.session.Query(`
		INSERT INTO workflow_runs (
			cluster_id, id, correlation_id, parent_run_id,
			node_id, node_hostname, component_name, component_kind, component_version,
			release_kind, release_object_id, desired_object_id, plan_id, plan_generation,
			trigger_reason, status, current_actor, failure_class,
			summary, error_message, retry_count,
			acknowledged, acknowledged_by, acknowledged_at,
			started_at, updated_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ctx.ClusterId, run.Id, run.CorrelationId, run.ParentRunId,
		ctx.NodeId, ctx.NodeHostname, ctx.ComponentName, int(ctx.ComponentKind), ctx.ComponentVersion,
		ctx.ReleaseKind, ctx.ReleaseObjectId, ctx.DesiredObjectId, ctx.PlanId, ctx.PlanGeneration,
		int(run.TriggerReason), int(run.Status), int(run.CurrentActor), int(run.FailureClass),
		run.Summary, run.ErrorMessage, run.RetryCount,
		run.Acknowledged, run.AcknowledgedBy, tsOrNil(run.AcknowledgedAt),
		tsToTime(run.StartedAt), tsToTime(run.UpdatedAt), tsOrNil(run.FinishedAt),
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
	now := time.Now()
	if err := srv.session.Query(`
		UPDATE workflow_runs SET status=?, summary=?, plan_id=?, plan_generation=?, current_actor=?, updated_at=?
		WHERE cluster_id=? AND started_at=(SELECT started_at FROM workflow_runs WHERE cluster_id=? AND id=? LIMIT 1 ALLOW FILTERING) AND id=?`,
		int(req.Status), req.Summary, req.PlanId, req.PlanGeneration, int(req.CurrentActor), now,
		req.ClusterId, req.ClusterId, req.Id, req.Id,
	).Exec(); err != nil {
		// ScyllaDB doesn't support subqueries in UPDATE WHERE. Use a two-step approach.
		return nil, srv.updateRunByID(req.ClusterId, req.Id, func(startedAt time.Time) error {
			return srv.session.Query(`
				UPDATE workflow_runs SET status=?, summary=?, plan_id=?, plan_generation=?, current_actor=?, updated_at=?
				WHERE cluster_id=? AND started_at=? AND id=?`,
				int(req.Status), req.Summary, req.PlanId, req.PlanGeneration, int(req.CurrentActor), now,
				req.ClusterId, startedAt, req.Id,
			).Exec()
		})
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) FinishRun(_ context.Context, req *workflowpb.FinishRunRequest) (*emptypb.Empty, error) {
	now := time.Now()
	err := srv.updateRunByID(req.ClusterId, req.Id, func(startedAt time.Time) error {
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
	}
	return &emptypb.Empty{}, err
}

func (srv *server) RecordStep(_ context.Context, req *workflowpb.RecordStepRequest) (*workflowpb.WorkflowStep, error) {
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
		filtered = append(filtered, r)
	}

	return &workflowpb.ListRunsResponse{
		Runs:  filtered,
		Total: int32(len(filtered)),
	}, nil
}

func (srv *server) GetRunEvents(_ context.Context, req *workflowpb.GetRunEventsRequest) (*workflowpb.GetRunEventsResponse, error) {
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
	return srv.ListRuns(ctx, &workflowpb.ListRunsRequest{
		ClusterId:  req.ClusterId,
		NodeId:     req.NodeId,
		ActiveOnly: true,
		Limit:      20,
	})
}

func (srv *server) GetComponentHistory(ctx context.Context, req *workflowpb.GetComponentHistoryRequest) (*workflowpb.ListRunsResponse, error) {
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
		id, corrID, parentID                                                                 string
		nodeID, nodeHostname, compName, compVersion                                          string
		relKind, relObjID, desObjID, planID                                                  string
		summary, errMsg, ackBy                                                               string
		compKind, trigReason, status, curActor, failClass, planGen, retryCnt                 int
		ack                                                                                  bool
		startedAt, updatedAt, finishedAt, ackAt                                              time.Time
	)

	if err := srv.session.Query(`
		SELECT id, correlation_id, parent_run_id,
			node_id, node_hostname, component_name, component_kind, component_version,
			release_kind, release_object_id, desired_object_id, plan_id, plan_generation,
			trigger_reason, status, current_actor, failure_class,
			summary, error_message, retry_count,
			acknowledged, acknowledged_by, acknowledged_at,
			started_at, updated_at, finished_at
		FROM workflow_runs WHERE cluster_id=? AND id=? LIMIT 1 ALLOW FILTERING`,
		clusterID, runID,
	).Scan(
		&id, &corrID, &parentID,
		&nodeID, &nodeHostname, &compName, &compKind, &compVersion,
		&relKind, &relObjID, &desObjID, &planID, &planGen,
		&trigReason, &status, &curActor, &failClass,
		&summary, &errMsg, &retryCnt,
		&ack, &ackBy, &ackAt,
		&startedAt, &updatedAt, &finishedAt,
	); err != nil {
		return nil, fmt.Errorf("load run %s: %w", runID, err)
	}

	return &workflowpb.WorkflowRun{
		Id:            id,
		CorrelationId: corrID,
		ParentRunId:   parentID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:       clusterID,
			NodeId:          nodeID,
			NodeHostname:    nodeHostname,
			ComponentName:   compName,
			ComponentKind:   workflowpb.ComponentKind(compKind),
			ComponentVersion: compVersion,
			ReleaseKind:     relKind,
			ReleaseObjectId: relObjID,
			DesiredObjectId: desObjID,
			PlanId:          planID,
			PlanGeneration:  int32(planGen),
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
		aID, rID, name, ver, digest, nodeID                                    string
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
		workflowpb.RunStatus_RUN_STATUS_PLANNING,
		workflowpb.RunStatus_RUN_STATUS_WAITING_FOR_SLOT,
		workflowpb.RunStatus_RUN_STATUS_DISPATCHED,
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
	case workflowpb.WorkflowPhaseKind_PHASE_PLAN:
		return "Plan"
	case workflowpb.WorkflowPhaseKind_PHASE_DISPATCH:
		return "Dispatch"
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
		Name:            "workflow.WorkflowService",
		Proto:           "workflow.proto",
		Path:            func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         Version,
		PublisherID:     "localhost",
		Description:     "Workflow service — reconciliation workflow tracing and history",
		Keywords:        []string{"workflow", "reconciliation", "trace", "operations", "scylladb"},
		AllowAllOrigins: true,
		KeepAlive:       true,
		KeepUpToDate:    true,
		Process:         -1,
		ProxyProcess:    -1,
		Repositories:    make([]string, 0),
		Discoveries:     make([]string, 0),
		Dependencies:    []string{},
		Permissions:     make([]interface{}, 0),
		ScyllaHosts:     []string{"127.0.0.1"},
		ScyllaPort:      9042,
		ScyllaReplicationFactor: 1,
	}
}

func setupGrpcService(s *server) {
	workflowpb.RegisterWorkflowServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
}

func main() {
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
