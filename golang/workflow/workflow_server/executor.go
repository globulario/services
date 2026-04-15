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
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/compiler"
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

	// Pre-compile for receipt lookup in OnStepDone callback.
	cw, _, compileErr := compiler.Compile(ctx, def)
	if compileErr != nil {
		return nil, fmt.Errorf("compile definition %s: %w", req.WorkflowName, compileErr)
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
	// Safety net: advance_infra_joins is a no-op when invoked on the workflow
	// service (should be handled by the controller). This prevents "no handler"
	// failures if the controller callback router is missing after a restart.
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.advance_infra_joins", func(ctx context.Context, _ engine.ActionRequest) (*engine.ActionResult, error) {
		return &engine.ActionResult{OK: true, Message: "noop advance_infra_joins (workflow service)"}, nil
	})

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
		seqMu:     sync.Mutex{},
		seq:       0,
	}

	eng := &engine.Engine{
		Router: router,
		RunID:  runID, // match the executor's run ID so actor callbacks can find the registered Router
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			recorder.onStepDone(run, step)
			srv.metricsStep(time.Now())
			// MC-1: Write receipt if step has a receipt_key and succeeded.
			if step.Status == engine.StepSucceeded && cw != nil {
				if cs, ok := cw.Steps[step.ID]; ok && cs.Execution != nil && cs.Execution.ReceiptKey != "" {
					srv.writeStepReceipt(runID, step.ID, cs.Execution.ReceiptKey, step.Output)
				}
			}
		},
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
	// Extract context fields from workflow inputs so runs are searchable
	// by component, node, and trigger reason in the admin UI.
	compName, _ := inputs["package_name"].(string)
	if compName == "" {
		compName, _ = inputs["component_name"].(string)
	}
	// Fallback: derive service name from the release_name (e.g. "core@globular.io/dns" → "dns")
	if compName == "" {
		if rn, _ := inputs["release_name"].(string); rn != "" {
			if idx := strings.LastIndex(rn, "/"); idx >= 0 {
				compName = rn[idx+1:]
			}
		}
	}
	compVersion, _ := inputs["resolved_version"].(string)
	if compVersion == "" {
		compVersion, _ = inputs["version"].(string)
	}
	compKind := workflowpb.ComponentKind_COMPONENT_KIND_UNKNOWN
	if k, _ := inputs["package_kind"].(string); k != "" {
		switch strings.ToUpper(k) {
		case "SERVICE":
			compKind = workflowpb.ComponentKind_COMPONENT_KIND_SERVICE
		case "INFRASTRUCTURE":
			compKind = workflowpb.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE
		}
	}

	// Trigger reason: infer from inputs.
	triggerReason := workflowpb.TriggerReason_TRIGGER_REASON_UNKNOWN
	switch {
	case inputs["desired_hash"] != nil:
		triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_DESIRED_DRIFT
	case inputs["scope"] == "cluster":
		// cluster.reconcile workflows are drift-repair driven
		triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_REPAIR
	case inputs["trigger_reason"] != nil:
		// Explicit trigger from caller
		if tr, ok := inputs["trigger_reason"].(string); ok {
			switch strings.ToUpper(tr) {
			case "BOOTSTRAP":
				triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_BOOTSTRAP
			case "REPAIR":
				triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_REPAIR
			case "UPGRADE":
				triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_UPGRADE
			case "RETRY":
				triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_RETRY
			case "MANUAL":
				triggerReason = workflowpb.TriggerReason_TRIGGER_REASON_MANUAL
			}
		}
	}

	// Extract node context from inputs when available.
	nodeID, _ := inputs["node_id"].(string)
	nodeHostname, _ := inputs["node_hostname"].(string)
	// For release workflows, extract from candidate_nodes.
	if nodeID == "" {
		if nodes, ok := inputs["candidate_nodes"].([]any); ok {
			if len(nodes) == 1 {
				if n, ok := nodes[0].(string); ok {
					nodeID = n
				}
			} else if len(nodes) > 1 {
				// Multi-node: store count as hostname hint for the UI.
				nodeHostname = fmt.Sprintf("%d nodes", len(nodes))
			}
		}
	}

	startRun := &workflowpb.WorkflowRun{
		Id:            runID,
		CorrelationId: req.CorrelationId,
		Context: &workflowpb.WorkflowContext{
			ClusterId:        req.ClusterId,
			NodeId:           nodeID,
			NodeHostname:     nodeHostname,
			ComponentName:    compName,
			ComponentVersion: compVersion,
			ComponentKind:    compKind,
		},
		TriggerReason: triggerReason,
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
	srv.metricsRunStart(runID, time.Now())

	logger.Info("executor: engine.Execute starting", "run_id", runID, "steps", len(def.Spec.Steps))
	run, execErr := eng.Execute(ctx, def, inputs)
	if execErr != nil {
		logger.Warn("executor: engine.Execute returned error", "run_id", runID, "error", execErr.Error())
	} else {
		logger.Info("executor: engine.Execute completed", "run_id", runID)
	}

	// ── 7. Record run finish ─────────────────────────────────────────────
	status := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	var errMsg string
	if run != nil && run.BlockedStepID != "" {
		// MC-3: Run is blocked waiting for operator approval.
		status = workflowpb.RunStatus_RUN_STATUS_BLOCKED
		errMsg = run.BlockedReason
	} else if execErr != nil {
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
	srv.metricsRunFinish(runID, status, time.Now())

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

	// AL-1: Project incident to ai-memory on FAILED/BLOCKED runs.
	// Fire-and-forget — learning must never block workflow response.
	go srv.projectIncident(context.Background(), req, resp)

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

	if tlsErr := config.ProbeTLS(dt.Address); tlsErr != nil {
		return nil, fmt.Errorf("dial %s at %s: %w", actorType, dt.Address, tlsErr)
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
		slog.Info("executor: dispatching action",
			"actor", actorType, "action", req.Action,
			"run_id", req.RunID, "step_id", req.StepID)
		client, err := d.getClient(actorType)
		if err != nil {
			slog.Warn("executor: actor dial failed",
				"actor", actorType, "action", req.Action, "err", err)
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
