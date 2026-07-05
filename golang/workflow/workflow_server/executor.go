// executor.go implements the ExecuteWorkflow RPC — the centralized workflow
// executor described in docs/centralized-workflow-execution.md.
//
// The executor:
//  1. Loads the workflow definition from etcd (single source of truth)
//  2. Builds a remote-dispatch Router using RegisterFallback per actor
//  3. Runs the engine to completion
//  4. Auto-records runs/steps to ScyllaDB as execution proceeds
//  5. Dispatches actions to actor services via gRPC callbacks
//  6. Uses config.ResolveDialTarget for all actor dials
//
// @awareness namespace=globular.platform
// @awareness component=platform_workflow.executor
// @awareness file_role=centralized_workflow_execution_rpc_and_engine_wiring
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:intent.reconciliation.must_be_idempotent_and_bounded
// @awareness risk=critical
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

// startRunFn is the commit-first seam: it persists the durable run record
// before any side effect runs. Indirected through a package var so tests can
// force the commit to fail and assert that no actor is ever dispatched
// (TestExecuteWorkflow_StartRunCommitFailure_NoSideEffects). Production wiring
// calls srv.StartRun.
var startRunFn = func(srv *server, ctx context.Context, req *workflowpb.StartRunRequest) (*workflowpb.WorkflowRun, error) {
	return srv.StartRun(ctx, req)
}

// ExecuteWorkflow loads a workflow definition from etcd, builds a remote-
// dispatch router for actor callbacks, runs the engine, and auto-records
// the entire run to ScyllaDB.
func (srv *server) ExecuteWorkflow(ctx context.Context, req *workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req.WorkflowName == "" {
		return nil, fmt.Errorf("workflow_name is required")
	}
	if req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}

	// ── 1. Load definition: etcd → local disk (bootstrap fallback) ───────
	// All workflow definitions live in etcd under /globular/workflows/.
	// Local disk (/var/lib/globular/workflows/) is a bootstrap fallback
	// for the window before SeedCoreWorkflows has run on a new cluster.
	var defYAML []byte
	if v1alpha1.EtcdFetcher != nil {
		if b, ferr := v1alpha1.EtcdFetcher(req.WorkflowName); ferr == nil && len(b) > 0 {
			defYAML = b
		}
	}
	if len(defYAML) == 0 {
		for _, path := range []string{
			"/var/lib/globular/workflows/" + req.WorkflowName,
			"/usr/lib/globular/workflows/" + req.WorkflowName,
		} {
			if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
				defYAML = b
				break
			}
		}
	}
	if len(defYAML) == 0 {
		return nil, fmt.Errorf("workflow definition %q not found (etcd + local disk checked)", req.WorkflowName)
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

	// ── WF-DEFER B3: persistent abandonment check. If this correlation
	// has hit MaxDefers across runs, refuse dispatch permanently
	// (until an operator clears the row). Per the awareness invariants
	// `convergence.no_infinite_retry` and the awareness pattern of a
	// per-correlation circuit breaker (NOT a global one — other
	// correlations continue dispatching unaffected).
	if req.CorrelationId != "" && srv.deferStore != nil {
		if state, err := srv.deferStore.Get(ctx, req.ClusterId, req.CorrelationId); err == nil && shouldSkipForAbandoned(state) {
			logger.Warn("executor: refusing dispatch — correlation ABANDONED after max defers",
				"workflow", req.WorkflowName,
				"correlation_id", req.CorrelationId,
				"defer_count", state.DeferCount,
				"max_defers", state.MaxDefers,
				"last_step_id", state.LastStepID,
				"last_reason", state.LastReason,
				"last_blocker_tags", state.LastBlockerTags,
				"abandoned_at", state.AbandonedAt,
				"operator_action", "globular workflow defer-state clear --correlation-id "+req.CorrelationId,
			)
			return &workflowpb.ExecuteWorkflowResponse{
				RunId:  req.CorrelationId,
				Status: workflowpb.RunStatus_RUN_STATUS_FAILED.String(),
				Error: fmt.Sprintf("ABANDONED after %d/%d defers on step %s: %s — operator must clear with `globular workflow defer-state clear --correlation-id %s`",
					state.DeferCount, state.MaxDefers, state.LastStepID, state.LastReason, req.CorrelationId),
			}, nil
		}
	}

	// ── WF-DEFER B2: skip if a prior run for this correlation_id is still
	// in defer cooldown. Preserves all other dispatch ordering — only
	// fires when both correlation_id is set AND a deferred record is
	// found whose backoff hasn't elapsed.
	if req.CorrelationId != "" {
		if dr := srv.findActiveDeferredRun(req.ClusterId, req.CorrelationId, time.Now()); dr != nil {
			logger.Info("executor: skipping dispatch — prior run is deferred",
				"workflow", req.WorkflowName,
				"correlation_id", req.CorrelationId,
				"deferred_run_id", dr.GetId(),
				"backoff_until_ms", dr.GetBackoffUntilMs(),
				"defer_count", dr.GetRetryAttempt(),
			)
			return &workflowpb.ExecuteWorkflowResponse{
				RunId:  dr.GetId(),
				Status: dr.GetStatus().String(),
				Error: fmt.Sprintf("dispatch skipped: prior run %s deferred until %d (defer_count=%d): %s",
					dr.GetId(), dr.GetBackoffUntilMs(), dr.GetRetryAttempt(), dr.GetErrorMessage()),
			}, nil
		}
	}

	// ── 2. Deserialize inputs ────────────────────────────────────────────
	inputs := make(map[string]any)
	if req.InputsJson != "" {
		if err := json.Unmarshal([]byte(req.InputsJson), &inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs_json: %w", err)
		}
	}

	// ── 3. Build remote-dispatch router ──────────────────────────────────
	// detached is set true by the ack-on-dispatch path below. When set, the
	// dispatcher and the lease are owned by the background goroutine, so these
	// RPC-return defers must NOT fire on the immediate return — they would close
	// the actor connections and release the lease out from under the still-running
	// detached execution.
	detached := false
	dispatcher := newActorDispatcher(req.ActorEndpoints)
	defer func() {
		if !detached {
			dispatcher.close()
		}
	}()

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
		seqMu:     sync.Mutex{},
		seq:       0,
	}

	eng := &engine.Engine{
		Router: router,
		RunID:  runID, // match the executor's run ID so actor callbacks can find the registered Router
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			recorder.onStepDone(run, step)
			srv.metricsStep(time.Now())
			// MC-1/blocked semantics: write receipts with structured status.
			// Success writes with explicit receipt_key; deterministic/transient
			// failures also persist receipt payloads so resume/AI can reason
			// from failure_class/reason/unblock_signals.
			if cw != nil {
				receiptKey := ""
				if cs, ok := cw.Steps[step.ID]; ok && cs.Execution != nil && cs.Execution.ReceiptKey != "" {
					receiptKey = cs.Execution.ReceiptKey
				}
				if receiptKey == "" && step.Error != "" {
					// Auto-key blocked/failed receipts so deterministic blockers
					// are persisted even for steps without explicit receipt_key.
					receiptKey = "auto_step_outcome:" + step.ID
				}
				if receiptKey != "" {
					srv.writeStepReceipt(runID, step.ID, receiptKey, buildStepReceiptPayload(step))
				}
			}
		},
	}

	// ── 5. Claim run ownership ───────────────────────────────────────────
	if srv.leaseManager != nil {
		claimed, err := srv.leaseManager.ClaimRun(ctx, runID)
		if err != nil {
			// Lease claim failure means the fencing mechanism is unavailable.
			// Proceeding without a fence risks double-execution. Refuse.
			// See meta.state_mutations_must_be_durably_committed_before_side_effects.
			return nil, fmt.Errorf("run %s: lease claim failed — refusing unfenced execution: %w", runID, err)
		} else if !claimed {
			return nil, fmt.Errorf("run %s already owned by another executor", runID)
		}
		defer func() {
			if !detached {
				srv.leaseManager.ReleaseRun(runID)
			}
		}()
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
	// Commit-first — meta.state_mutations_must_be_durably_committed_before_side_effects.
	// StartRun persists the workflow service's durable run record: the
	// coordinator's intent + per-step progress + resume point. It MUST commit
	// before any actor side effect runs. Dispatching steps without this record
	// is the "lost intent / untraceable change" class — the effects happen, no
	// run record explains them, and a workflow-service restart cannot resume or
	// audit the run. On failure we refuse to execute and return an error so the
	// caller retries the whole dispatch (retry is the only response). StartRun
	// is an idempotent upsert (CQL INSERT by primary key + supersedePriorRuns)
	// and engine steps are idempotent (reconciliation.must_be_idempotent_and_bounded),
	// so a retry that re-commits and re-executes does not double-apply. The lease
	// claimed above is freed by the deferred ReleaseRun.
	if _, err := startRunFn(srv, ctx, &workflowpb.StartRunRequest{Run: startRun}); err != nil {
		logger.Error("executor: run-start commit failed — refusing to dispatch side effects uncommitted",
			"run_id", runID, "err", err)
		return nil, fmt.Errorf("workflow %s: run-start commit failed, refusing to execute uncommitted: %w", req.WorkflowName, err)
	}

	// Persist the run inputs as part of the commit, before any side effect.
	// An orphan resume rebuilds the engine and must re-execute the remaining
	// steps against the SAME inputs the run started with — without this the
	// resume runs with an empty map and every inputs.* reference (candidate
	// nodes, desired_hash, version, package_kind) silently resolves to nil.
	// Gated like the run-start commit: if it fails we refuse to dispatch so
	// the caller retries the whole (idempotent) dispatch.
	// (meta.binding_outlives_evidence_until_invalidated)
	if req.InputsJson != "" {
		if err := srv.persistRunInputs(req.ClusterId, tsToTime(now), runID, req.InputsJson); err != nil {
			logger.Error("executor: run-inputs commit failed — refusing to dispatch side effects uncommitted",
				"run_id", runID, "err", err)
			return nil, fmt.Errorf("workflow %s: run-inputs commit failed, refusing to execute uncommitted: %w", req.WorkflowName, err)
		}
	}

	// ── 6. Execute ───────────────────────────────────────────────────────
	// runToCompletion runs the engine DAG, finalizes the run (FinishRun with
	// retry), clears defer state, and builds the response. Shared by the
	// synchronous path and the detached background goroutine; it takes the
	// execution context explicitly because detached must run on a FRESH context
	// (the request ctx is cancelled the moment ExecuteWorkflow returns). Every
	// other value (def/inputs/eng/cw/runID/req/logger/srv) is captured.
	runToCompletion := func(ctx context.Context) *workflowpb.ExecuteWorkflowResponse {
		logger.Info("executor: starting workflow",
			"workflow", req.WorkflowName, "run_id", runID,
			"actors", fmt.Sprintf("%v", mapKeys(req.ActorEndpoints)))
		srv.metricsRunStart(runID, req.WorkflowName, time.Now())

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
		if run != nil && run.Status == engine.RunDeferred && run.Defer != nil {
			// WF-DEFER B2: a step yielded its slot. Persist defer_until +
			// defer_count via a direct UPDATE — FinishRun's path is for
			// terminal/blocked outcomes and overwriting the row here would
			// drop our backoff fields.
			status = workflowpb.RunStatus_RUN_STATUS_DEFERRED
			errMsg = run.Defer.Reason
			if rerr := srv.recordRunDeferred(ctx, req.ClusterId, runID, run.Defer); rerr != nil {
				logger.Warn("executor: failed to record deferred run", "run_id", runID, "err", rerr)
			}
			// WF-DEFER B3: increment the persistent across-runs counter.
			// Once defer_count >= max_defers, the store flips abandoned=true
			// and the next dispatch attempt for this correlation_id is
			// refused permanently (until operator clear).
			if req.CorrelationId != "" && srv.deferStore != nil {
				if state, derr := srv.deferStore.RecordDefer(ctx, req.ClusterId, req.CorrelationId, run.Defer); derr != nil {
					logger.Warn("executor: failed to record persistent defer state",
						"correlation_id", req.CorrelationId, "err", derr)
				} else if state != nil {
					logger.Info("executor: defer state updated",
						"correlation_id", req.CorrelationId,
						"defer_count", state.DeferCount,
						"max_defers", state.MaxDefers,
						"abandoned", state.Abandoned,
						"last_step_id", state.LastStepID,
						"last_blocker_tags", state.LastBlockerTags,
					)
					if state.Abandoned {
						logger.Warn("executor: correlation now ABANDONED — auto-retry stopped, operator action required",
							"correlation_id", req.CorrelationId,
							"defer_count", state.DeferCount,
							"max_defers", state.MaxDefers,
							"last_step_id", state.LastStepID,
							"last_reason", state.LastReason,
						)
						publishWorkflowEvent("workflow.correlation.abandoned", map[string]interface{}{
							"correlation_id":    req.CorrelationId,
							"workflow":          req.WorkflowName,
							"defer_count":       state.DeferCount,
							"max_defers":        state.MaxDefers,
							"last_step_id":      state.LastStepID,
							"last_reason":       state.LastReason,
							"last_blocker_tags": state.LastBlockerTags,
						})
					}
				}
			}
		} else if run != nil && run.BlockedStepID != "" {
			// MC-3: Run is blocked waiting for operator approval.
			status = workflowpb.RunStatus_RUN_STATUS_BLOCKED
			errMsg = run.BlockedReason
		} else if execErr != nil {
			status = workflowpb.RunStatus_RUN_STATUS_FAILED
			errMsg = execErr.Error()
		}

		summary := fmt.Sprintf("%s: %s", req.WorkflowName, status.String())
		// FinishRun handles terminal+BLOCKED status writes. DEFERRED was
		// already persisted above with backoff fields; calling FinishRun
		// here would overwrite the row without those fields.
		if status != workflowpb.RunStatus_RUN_STATUS_DEFERRED {
			// The terminal close must survive (a) a cancelled/expired request context
			// — under load the caller's ExecuteWorkflow deadline can elapse before the
			// DAG finishes, and closing on that dead ctx silently fails — and (b) a
			// transient ScyllaDB blip. Otherwise the run is stranded non-terminal
			// (workflow.must_reach_terminal_state), which is exactly the hung-run
			// symptom. FinishRun is idempotent (server.go terminal-status guard), so
			// bounded retry on a FRESH context is safe. If every attempt fails, the
			// reaper's progress-deadline path is the durable backstop.
			finishReq := &workflowpb.FinishRunRequest{
				Id:           runID,
				ClusterId:    req.ClusterId,
				Status:       status,
				Summary:      summary,
				ErrorMessage: errMsg,
			}
			var finishErr error
			for attempt := 1; attempt <= 3; attempt++ {
				fctx, fcancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, finishErr = srv.FinishRun(fctx, finishReq)
				fcancel()
				if finishErr == nil {
					break
				}
				logger.Warn("executor: FinishRun attempt failed, retrying",
					"run_id", runID, "attempt", attempt, "err", finishErr)
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			}
			if finishErr != nil {
				logger.Error("executor: FinishRun failed after retries — run left non-terminal for reaper to finalize",
					"run_id", runID, "status", status.String(), "err", finishErr)
			}
		}

		// WF-DEFER B3: clear the persistent defer counter on a clean
		// success. A correlation that deferred N times and finally
		// converged should get a full retry budget on its NEXT failure
		// rather than starting at N — otherwise a single transient blip
		// during the year could quietly burn the whole budget. Hard
		// failures leave the counter alone (a deferred condition is a
		// distinct mode from a hard failure mode).
		if status == workflowpb.RunStatus_RUN_STATUS_SUCCEEDED &&
			req.CorrelationId != "" && srv.deferStore != nil {
			if cerr := srv.deferStore.ClearOnSuccess(ctx, req.ClusterId, req.CorrelationId); cerr != nil {
				logger.Warn("executor: failed to clear persistent defer state on success",
					"correlation_id", req.CorrelationId, "err", cerr)
			}
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
		// Fire-and-forget with a bounded timeout — learning must never block
		// workflow response, but unbounded goroutines must not accumulate when
		// ai-memory is slow. See meta.failure_response_must_contract_not_amplify.
		incidentCtx, incidentCancel := context.WithTimeout(context.Background(), 10*time.Second)
		go func() {
			defer incidentCancel()
			srv.projectIncident(incidentCtx, req, resp)
		}()

		return resp
	}

	// Detached (ack-on-dispatch): the run row, its inputs, and the lease are
	// durably committed and claimed above. Execute the DAG in the background on a
	// FRESH context and return EXECUTING immediately, so the caller never blocks
	// for the run duration — a long run then cannot time out the dispatch RPC and
	// open the caller's workflow circuit breaker. The dispatcher and lease are
	// released inside the goroutine (the RPC-return defers are suppressed via
	// `detached`). Opt-in; callers MUST NOT set detached for child workflows —
	// wait_child_terminal needs a terminal child.
	if req.Detached {
		detached = true
		go func() {
			defer dispatcher.close()
			if srv.leaseManager != nil {
				defer srv.leaseManager.ReleaseRun(runID)
			}
			bgCtx, bgCancel := context.WithCancel(context.Background())
			defer bgCancel()
			runToCompletion(bgCtx)
		}()
		return &workflowpb.ExecuteWorkflowResponse{
			RunId:  runID,
			Status: workflowpb.RunStatus_RUN_STATUS_EXECUTING.String(),
		}, nil
	}

	return runToCompletion(ctx), nil
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

	// Stamp run progress on the executor lease. This is the signal that lets the
	// reaper tell "alive and advancing" from "alive but hung": every completed
	// step refreshes last_progress_at, so a run whose executor stops advancing
	// (but keeps heartbeating) goes stale and becomes recoverable.
	if r.srv.leaseManager != nil {
		r.srv.leaseManager.RecordProgress(r.runID)
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
