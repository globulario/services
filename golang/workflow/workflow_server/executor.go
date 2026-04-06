// executor.go implements the ExecuteWorkflow RPC — the centralized workflow
// executor described in docs/centralized-workflow-execution.md.
//
// The executor:
//  1. Loads the workflow definition from MinIO (single source of truth)
//  2. Builds a remote-dispatch Router using RegisterFallback per actor
//  3. Runs the engine to completion
//  4. Auto-records runs/steps to ScyllaDB as execution proceeds
//  5. Dispatches actions to actor services via gRPC callbacks
//  6. Uses config.ResolveDialTarget for all actor dials
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"

	"github.com/gocql/gocql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ExecuteWorkflow loads a workflow definition from MinIO, builds a remote-
// dispatch router for actor callbacks, runs the engine, and auto-records
// the entire run to ScyllaDB.
func (srv *server) ExecuteWorkflow(ctx context.Context, req *workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
	if req.WorkflowName == "" {
		return nil, fmt.Errorf("workflow_name is required")
	}
	if req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}

	// ── 1. Load definition from MinIO ────────────────────────────────────
	defYAML, err := config.GetClusterConfig("workflows/" + req.WorkflowName + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("load definition %s: %w", req.WorkflowName, err)
	}
	if defYAML == nil {
		return nil, fmt.Errorf("workflow definition %q not found in MinIO", req.WorkflowName)
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadBytes(defYAML)
	if err != nil {
		return nil, fmt.Errorf("parse definition %s: %w", req.WorkflowName, err)
	}

	// ── 2. Deserialize inputs ────────────────────────────────────────────
	inputs := make(map[string]any)
	if req.InputsJson != "" {
		if err := json.Unmarshal([]byte(req.InputsJson), &inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs_json: %w", err)
		}
	}

	// ── 3. Build remote-dispatch router ──────────────────────────────────
	dispatcher := newActorDispatcher(req.ActorEndpoints)
	defer dispatcher.close()

	router := engine.NewRouter()

	// Register workflow-service as a local actor (self-dispatch for child
	// workflows and drift tracking). Uses a no-op config for now — these
	// actions are only used by cluster.reconcile which is Phase E.
	engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{})

	// Register fallback handlers for all remote actors. The fallback is
	// transport-only: it marshals the ActionRequest to gRPC and calls the
	// actor's WorkflowActorService.ExecuteAction endpoint.
	for actorType := range req.ActorEndpoints {
		at := actorType // capture
		router.RegisterFallback(v1alpha1.ActorType(at), dispatcher.makeHandler(at))
	}

	// ── 4. Build engine with auto-recording ──────────────────────────────
	// Use correlation_id as the run_id if provided. This allows callers
	// (e.g. cluster-controller) to register per-run actor Routers keyed
	// by correlation_id before the call, since they can predict the run_id.
	runID := req.CorrelationId
	if runID == "" {
		runID = gocql.TimeUUID().String()
	}
	recorder := &executionRecorder{
		srv:       srv,
		clusterID: req.ClusterId,
		runID:     runID,
		seqMu:    sync.Mutex{},
		seq:       0,
	}

	eng := &engine.Engine{
		Router:     router,
		OnStepDone: recorder.onStepDone,
	}

	// ── 5. Claim run ownership ───────────────────────────────────────────
	if srv.leaseManager != nil {
		claimed, err := srv.leaseManager.ClaimRun(ctx, runID)
		if err != nil {
			logger.Warn("executor: lease claim failed (proceeding anyway)", "run_id", runID, "err", err)
		} else if !claimed {
			return nil, fmt.Errorf("run %s already owned by another executor", runID)
		}
		defer srv.leaseManager.ReleaseRun(runID)
	}

	// ── 6. Record run start ──────────────────────────────────────────────
	now := timestamppb.Now()
	startRun := &workflowpb.WorkflowRun{
		Id:            runID,
		CorrelationId: req.CorrelationId,
		Context: &workflowpb.WorkflowContext{
			ClusterId: req.ClusterId,
		},
		TriggerReason: workflowpb.TriggerReason_TRIGGER_REASON_MANUAL,
		Status:        workflowpb.RunStatus_RUN_STATUS_EXECUTING,
		CurrentActor:  workflowpb.WorkflowActor_ACTOR_WORKFLOW_SERVICE,
		StartedAt:     now,
		WorkflowName:  req.WorkflowName,
	}
	if _, err := srv.StartRun(ctx, &workflowpb.StartRunRequest{Run: startRun}); err != nil {
		logger.Warn("executor: failed to record run start", "run_id", runID, "err", err)
		// Non-fatal: execution proceeds even if recording fails.
	}

	// ── 6. Execute ───────────────────────────────────────────────────────
	logger.Info("executor: starting workflow",
		"workflow", req.WorkflowName, "run_id", runID,
		"actors", fmt.Sprintf("%v", mapKeys(req.ActorEndpoints)))

	run, execErr := eng.Execute(ctx, def, inputs)

	// ── 7. Record run finish ─────────────────────────────────────────────
	status := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	var errMsg string
	if execErr != nil {
		status = workflowpb.RunStatus_RUN_STATUS_FAILED
		errMsg = execErr.Error()
	}

	summary := fmt.Sprintf("%s: %s", req.WorkflowName, status.String())
	if _, err := srv.FinishRun(ctx, &workflowpb.FinishRunRequest{
		Id:           runID,
		ClusterId:    req.ClusterId,
		Status:       status,
		Summary:      summary,
		ErrorMessage: errMsg,
	}); err != nil {
		logger.Warn("executor: failed to record run finish", "run_id", runID, "err", err)
	}

	logger.Info("executor: workflow finished",
		"workflow", req.WorkflowName, "run_id", runID,
		"status", status.String())

	// ── 8. Build response ────────────────────────────────────────────────
	resp := &workflowpb.ExecuteWorkflowResponse{
		RunId:  runID,
		Status: status.String(),
		Error:  errMsg,
	}
	if run != nil && run.Outputs != nil {
		if b, err := json.Marshal(run.Outputs); err == nil {
			resp.OutputsJson = string(b)
		}
	}
	return resp, nil
}

// ─── Actor dispatcher ────────────────────────────────────────────────────────

// actorDispatcher manages gRPC connections to actor callback endpoints.
// All dials go through config.ResolveDialTarget for TLS safety.
type actorDispatcher struct {
	endpoints map[string]string // actor_type → raw endpoint
	mu        sync.Mutex
	conns     map[string]*grpc.ClientConn
	clients   map[string]workflowpb.WorkflowActorServiceClient
}

func newActorDispatcher(endpoints map[string]string) *actorDispatcher {
	return &actorDispatcher{
		endpoints: endpoints,
		conns:     make(map[string]*grpc.ClientConn),
		clients:   make(map[string]workflowpb.WorkflowActorServiceClient),
	}
}

func (d *actorDispatcher) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, conn := range d.conns {
		conn.Close()
	}
}

// getClient returns a cached or newly-created gRPC client for the given actor.
func (d *actorDispatcher) getClient(actorType string) (workflowpb.WorkflowActorServiceClient, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if c, ok := d.clients[actorType]; ok {
		return c, nil
	}

	raw, ok := d.endpoints[actorType]
	if !ok || raw == "" {
		return nil, fmt.Errorf("no endpoint configured for actor %q", actorType)
	}

	// Canonical endpoint resolution — no ad-hoc loopback rewrites.
	dt := config.ResolveDialTarget(raw)
	if dt.Address == "" {
		return nil, fmt.Errorf("ResolveDialTarget returned empty address for %q", raw)
	}

	creds, err := loadExecutorTLS(dt.ServerName)
	if err != nil {
		return nil, fmt.Errorf("load TLS for %s: %w", actorType, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, dt.Address,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s at %s: %w", actorType, dt.Address, err)
	}

	client := workflowpb.NewWorkflowActorServiceClient(conn)
	d.conns[actorType] = conn
	d.clients[actorType] = client
	return client, nil
}

// makeHandler returns an engine.ActionHandler that dispatches the action to
// the remote actor via gRPC. This is a transport-only fallback — the actor
// validates the action name and rejects unknowns.
func (d *actorDispatcher) makeHandler(actorType string) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		client, err := d.getClient(actorType)
		if err != nil {
			return nil, fmt.Errorf("actor %s: %w", actorType, err)
		}

		withJSON, _ := json.Marshal(req.With)
		inputsJSON, _ := json.Marshal(req.Inputs)
		outputsJSON, _ := json.Marshal(req.Outputs)

		resp, err := client.ExecuteAction(ctx, &workflowpb.ExecuteActionRequest{
			RunId:       req.RunID,
			StepId:      req.StepID,
			Actor:       actorType,
			Action:      req.Action,
			WithJson:    string(withJSON),
			InputsJson:  string(inputsJSON),
			OutputsJson: string(outputsJSON),
		})
		if err != nil {
			return nil, fmt.Errorf("actor %s action %s: %w", actorType, req.Action, err)
		}

		if !resp.Ok {
			return nil, fmt.Errorf("actor %s action %s rejected: %s", actorType, req.Action, resp.Message)
		}

		var output map[string]any
		if resp.OutputJson != "" {
			if err := json.Unmarshal([]byte(resp.OutputJson), &output); err != nil {
				slog.Warn("executor: failed to unmarshal action output",
					"actor", actorType, "action", req.Action, "err", err)
			}
		}

		return &engine.ActionResult{
			OK:      true,
			Output:  output,
			Message: resp.Message,
		}, nil
	}
}

// loadExecutorTLS loads service TLS credentials for actor callbacks.
func loadExecutorTLS(serverName string) (credentials.TransportCredentials, error) {
	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	caFile := "/var/lib/globular/pki/ca.crt"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   serverName,
	}), nil
}

// ─── Execution recorder ─────────────────────────────────────────────────────

// executionRecorder implements engine.OnStepDone to auto-record step progress
// to ScyllaDB during workflow execution. This replaces the external Recorder
// for workflows executed via ExecuteWorkflow.
type executionRecorder struct {
	srv       *server
	clusterID string
	runID     string
	seqMu     sync.Mutex
	seq       int32
}

func (r *executionRecorder) nextSeq() int32 {
	r.seqMu.Lock()
	defer r.seqMu.Unlock()
	r.seq++
	return r.seq
}

func (r *executionRecorder) onStepDone(run *engine.Run, step *engine.StepState) {
	seq := r.nextSeq()
	now := timestamppb.Now()

	status := workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
	var errMsg string
	switch step.Status {
	case engine.StepFailed:
		status = workflowpb.StepStatus_STEP_STATUS_FAILED
		errMsg = step.Error
	case engine.StepSkipped:
		status = workflowpb.StepStatus_STEP_STATUS_SKIPPED
	case engine.StepRunning:
		status = workflowpb.StepStatus_STEP_STATUS_RUNNING
	case engine.StepPending:
		status = workflowpb.StepStatus_STEP_STATUS_PENDING
	}

	durationMs := int64(0)
	if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
		durationMs = step.FinishedAt.Sub(step.StartedAt).Milliseconds()
	}

	var startedAt *timestamppb.Timestamp
	if !step.StartedAt.IsZero() {
		startedAt = timestamppb.New(step.StartedAt)
	}
	var finishedAt *timestamppb.Timestamp
	if !step.FinishedAt.IsZero() {
		finishedAt = timestamppb.New(step.FinishedAt)
	}

	// Serialize step output as details_json for observability.
	var detailsJSON string
	if step.Output != nil {
		if b, err := json.Marshal(step.Output); err == nil {
			detailsJSON = string(b)
		}
	}

	wsStep := &workflowpb.WorkflowStep{
		RunId:        r.runID,
		Seq:          seq,
		StepKey:      step.ID,
		Title:        step.ID,
		Actor:        workflowpb.WorkflowActor_ACTOR_WORKFLOW_SERVICE,
		Status:       status,
		Attempt:      int32(step.Attempt),
		CreatedAt:    now,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		DurationMs:   durationMs,
		ErrorMessage: errMsg,
		DetailsJson:  detailsJSON,
	}

	if _, err := r.srv.RecordStep(context.Background(), &workflowpb.RecordStepRequest{
		ClusterId: r.clusterID,
		Step:      wsStep,
	}); err != nil {
		slog.Warn("executor: failed to record step",
			"run_id", r.runID, "step", step.ID, "err", err)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
