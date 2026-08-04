package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// remediationWorkflowName is the canonical definition name stored in MinIO
// under workflows/remediate.doctor.finding.yaml.
const remediationWorkflowName = "remediate.doctor.finding"

// RunRemediationWorkflow delegates execution to the centralized
// WorkflowService. The workflow service loads the definition from MinIO,
// runs the engine, and dispatches steps back to this doctor via the
// WorkflowActorService.ExecuteAction callback.
//
// All side-effects still go through the existing ExecuteRemediation
// handler (called back via the actor service) — behavioral semantics
// are unchanged.
func (s *ClusterDoctorServer) RunRemediationWorkflow(
	ctx context.Context,
	findingID string,
	stepIndex uint32,
	approvalToken string,
	dryRun bool,
) (*workflowpb.ExecuteWorkflowResponse, error) {
	if findingID == "" {
		return nil, fmt.Errorf("finding_id is required")
	}
	if s.workflowClient == nil {
		return nil, fmt.Errorf("workflow service not configured (workflow_endpoint not set)")
	}

	inputs := map[string]any{
		"finding_id":     findingID,
		"step_index":     stepIndex,
		"approval_token": approvalToken,
		"dry_run":        dryRun,
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("marshal inputs: %w", err)
	}

	// Same routable-callback rule as the autonomous path. This backs the public
	// StartRemediationWorkflow RPC, so an operator-triggered run placed on a
	// Workflow Service on another node would have been just as unreachable —
	// "localhost" names the workflow node's own port, not this doctor.
	doctorEndpoint, err := s.resolveActorEndpoint()
	if err != nil {
		return nil, err
	}

	resp, err := s.workflowClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    s.clusterID,
		WorkflowName: remediationWorkflowName,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-doctor": doctorEndpoint,
		},
		CorrelationId: fmt.Sprintf("remediation/%s/%d", findingID, stepIndex),
	})
	if err != nil {
		return nil, fmt.Errorf("execute workflow via WorkflowService: %w", err)
	}

	return resp, nil
}

// runAutonomousRemediation starts one real remediate.doctor.finding run for a
// finding the healer selected, and derives the dispatch disposition from that
// run's own outputs.
//
// Identity, in two parts that must not be one string:
//
//   - runID is fresh per attempt. It is generated HERE, before the call, so the
//     exact finding can be bound to it before the workflow can call back, and
//     so a repeated repair of the same finding is a distinct run rather than an
//     alias of the first. Passing a stable id instead would make the second
//     attempt fail the run lease outright ("already owned by another executor").
//   - correlationID is stable across attempts on the same finding+step, which
//     is what joins retries into one story.
//
// The binding is written before ExecuteWorkflow and released on every exit
// path, including cancellation and panic unwinding.
func (s *ClusterDoctorServer) runAutonomousRemediation(
	ctx context.Context,
	f rules.Finding,
	stepIndex uint32,
	dryRun bool,
) (rules.DispatchResult, error) {
	if s.workflowClient == nil {
		// Fail closed. Executing directly would produce a mutation with no run
		// to attribute it to, no governance decision, and no recorded outcome.
		return rules.DispatchResult{}, fmt.Errorf(
			"workflow service not configured (workflow_endpoint not set) — refusing to remediate %s on %s "+
				"outside a governed run", f.InvariantID, f.EntityRef)
	}

	runID := uuid.New().String()
	correlationID := fmt.Sprintf("remediation/%s/%d", f.FindingID, stepIndex)

	if err := s.runFindings.bind(runID, f, stepIndex); err != nil {
		return rules.DispatchResult{}, fmt.Errorf("bind finding to run: %w", err)
	}
	defer s.runFindings.release(runID)

	inputs := map[string]any{
		"finding_id": f.FindingID,
		"step_index": stepIndex,
		// No approval token: an autonomous repair is authorized by the
		// workflow's behavioral-governance gate, not by an operator secret.
		"approval_token": "",
		"dry_run":        dryRun,
		// Explicit resolution contract: this run exists for ONE exact finding,
		// already bound to its run id. The resolver must demand that binding and
		// fail closed without it.
		"finding_binding_mode": engine.FindingBindingAutonomousRequired,
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return rules.DispatchResult{}, fmt.Errorf("marshal inputs: %w", err)
	}

	// The callback endpoint must be ROUTABLE, not loopback.
	//
	// Service discovery can place Workflow Service on any node. That service
	// resolves this address on ITS host, so "localhost" names the workflow
	// node's own port and the callback never reaches this doctor — every
	// autonomous run would die at its first actor step. It also violates the
	// no-localhost-for-remote-addresses rule outright.
	//
	// Taken from the registered service record rather than assembled from host
	// and port, so the address the workflow dials is the address this instance
	// actually published.
	actorEndpoint, err := s.resolveActorEndpoint()
	if err != nil {
		return rules.DispatchResult{}, err
	}

	resp, err := s.workflowClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    s.clusterID,
		WorkflowName: remediationWorkflowName,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-doctor": actorEndpoint,
		},
		RunId:         runID,
		CorrelationId: correlationID,
	})
	if err != nil {
		// WorkflowRunID stays EMPTY. The id we generated is a request, not a
		// receipt: after a transport failure we cannot tell whether Workflow
		// Service durably committed a run, and reporting the proposed id would
		// assert a durable run we have no evidence for. Ambiguity is reported as
		// ambiguity.
		return rules.DispatchResult{}, fmt.Errorf("start remediation workflow "+
			"(run %s proposed; durable commit UNCONFIRMED): %w", runID, err)
	}

	// A response only counts as a committed run if it names one, and names the
	// one we asked for. A missing id proves nothing was committed; a different
	// id means the run that executed is not the run we bound our finding to, so
	// nothing about that run's subject can be trusted.
	confirmedRunID := resp.GetRunId()
	if confirmedRunID == "" {
		return rules.DispatchResult{}, fmt.Errorf(
			"remediation workflow returned no run id (requested %s) — refusing to treat an "+
				"unconfirmed run as durable", runID)
	}
	if confirmedRunID != runID {
		return rules.DispatchResult{}, fmt.Errorf(
			"remediation workflow returned run id %s but %s was requested and bound — the executed "+
				"run is not the run this finding was bound to", confirmedRunID, runID)
	}

	return classifyRemediationRun(confirmedRunID, resp, dryRun), nil
}

// resolveActorEndpoint returns this doctor's registered, routable callback
// address, or an error.
//
// Injectable so transport tests can supply a bufconn target. Production leaves
// the hook nil and reads the canonical service registration.
//
// Fails closed BEFORE the workflow starts: a run dispatched with an
// unreachable callback burns a run id, a governance decision and a lease, then
// dies at its first actor step with a transport error that says nothing about
// the real cause.
func (s *ClusterDoctorServer) resolveActorEndpoint() (string, error) {
	resolve := s.actorEndpointResolver
	if resolve == nil {
		resolve = func() string { return config.ResolveLocalServiceAddr(doctorServiceName) }
	}
	addr := resolve()
	if addr == "" {
		return "", fmt.Errorf("no registered endpoint for %s — refusing to start a remediation run "+
			"the workflow service could not call back", doctorServiceName)
	}
	if err := rejectUnroutableCallback(addr); err != nil {
		return "", err
	}
	return addr, nil
}

// doctorServiceName is the registration this doctor publishes itself under.
const doctorServiceName = "cluster_doctor.ClusterDoctorService"

// rejectUnroutableCallback refuses addresses that only resolve on the caller's
// own host. Checked explicitly rather than trusted from the registry, because a
// misconfigured registration is exactly the case that produces a run which
// cannot be called back.
func rejectUnroutableCallback(addr string) error {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0", "[::]", "::":
		return fmt.Errorf("registered endpoint %q is not routable from another node — "+
			"a workflow service on a different host cannot call back to it", addr)
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		return fmt.Errorf("registered endpoint %q is loopback or unspecified — not routable", addr)
	}
	return nil
}

// classifyRemediationRun turns a finished workflow run into a dispatch
// disposition, reading the run's structured outputs only.
//
// Never infers from human-readable error text: a classifier that pattern-matches
// messages silently changes meaning whenever a message is reworded, and the
// distinction between "governance refused" and "executor broke" is exactly the
// one that must not blur — one is the system working, the other is the circuit
// breaker's business.
func classifyRemediationRun(runID string, resp *workflowpb.ExecuteWorkflowResponse, dryRun bool) rules.DispatchResult {
	res := rules.DispatchResult{WorkflowRunID: runID}

	outcome, err := remediationOutcomeFromRun(resp)
	if err != nil {
		// Classification itself failed. Reported as an execution failure rather
		// than guessed into any disposition.
		res.Disposition = rules.DispatchExecutionFailed
		res.Err = fmt.Errorf("remediation run %s: %w", runID, err)
		return res
	}
	res.AuditID = outcome.auditID
	res.ActionCheckID = outcome.actionCheckID
	res.GovernanceStatus = outcome.governanceStatus
	res.Executed = outcome.executed
	res.Verified = outcome.verified
	res.Converged = outcome.converged

	switch {
	case outcome.governanceUnavailable:
		// The governor could not be reached. An infrastructure failure, NOT a
		// decision — it must charge the circuit breaker so a governance outage
		// becomes visible instead of looking like routine refusals.
		res.Disposition = rules.DispatchExecutionFailed
		res.Err = fmt.Errorf("remediation run %s: governance unavailable", runID)
	case outcome.refused:
		// Governance declined before any side effect. Not a malfunction, so it
		// never charges the executor failure budget.
		res.Disposition = rules.DispatchRefused
	case dryRun, !outcome.executed && resp.GetStatus() == "SUCCEEDED":
		// A rehearsal, or a run that completed without attempting a mutation.
		res.Disposition = rules.DispatchProposed
	case !outcome.executed:
		res.Disposition = rules.DispatchExecutionFailed
		res.Err = fmt.Errorf("remediation run %s did not execute: status=%s error=%s",
			runID, resp.GetStatus(), resp.GetError())
	case !outcome.verified:
		// The mutation happened but no post-action evidence was obtained, so
		// convergence is unknown. Reported as its own state rather than guessed
		// in either direction.
		res.Disposition = rules.DispatchExecutedUnverified
		res.Err = fmt.Errorf("remediation run %s executed but convergence could not be verified: %s",
			runID, resp.GetError())
	case !outcome.converged:
		res.Disposition = rules.DispatchExecutedNotConverged
	case !outcome.hasDispatchInstant:
		// The finding cleared, but the run carries no dispatch instant, so the
		// convergence reading cannot be placed AFTER the action. Reporting
		// CONVERGED here would increment AutoFixed for a repair whose success
		// cannot be attributed — the same "the report lies" class this
		// architecture exists to remove.
		//
		// Defence in depth: the doctor's verifier already refuses a zero
		// dispatch instant. This ensures a future verifier that forgets to, or
		// a timestamp lost in transport, cannot manufacture an auto-fix.
		res.Disposition = rules.DispatchExecutedUnverified
		res.Err = fmt.Errorf("remediation run %s converged but carries no dispatch instant — "+
			"convergence cannot be placed after the action", runID)
	default:
		res.Disposition = rules.DispatchConverged
	}
	return res
}

// runRemediationOutcome is the subset of the run's remediation_outcome output
// the classifier needs.
type runRemediationOutcome struct {
	executed  bool
	verified  bool
	converged bool
	refused   bool
	auditID   string
	// actionCheckID is the real governance decision id, kept visible even when
	// the action was refused — a refusal an operator cannot trace back to its
	// decision is not accountable.
	actionCheckID string
	// governanceUnavailable distinguishes "the governor could not be reached"
	// from "the governor said no".
	governanceUnavailable bool
	// hasDispatchInstant reports whether the run recorded WHEN the executor
	// accepted. Without it, a convergence reading cannot be ordered after the
	// action it claims to verify.
	hasDispatchInstant bool
	// governanceStatus is the governor's structured status for a refusal
	// (e.g. needs_evidence), kept so an operator sees WHY without parsing prose.
	governanceStatus string
}

// remediationOutcomeFromRun reads the canonical remediation_outcome the verify
// step emitted into the run's outputs. Absent outputs leave every field false,
// which classifies as a failure rather than a success — the safe direction when
// a run's own account of itself is missing.
func remediationOutcomeFromRun(resp *workflowpb.ExecuteWorkflowResponse) (runRemediationOutcome, error) {
	var out runRemediationOutcome
	if resp == nil || resp.GetOutputsJson() == "" {
		return out, nil
	}
	var outputs map[string]any
	if err := json.Unmarshal([]byte(resp.GetOutputsJson()), &outputs); err != nil {
		return out, nil
	}
	// The canonical dispatch_result the execute step writes BEFORE returning on
	// a governed refusal or a governance-unavailable failure. It is read first
	// because those paths never reach execution_result at all: the step errors
	// out, and without this the refusal would be indistinguishable from an
	// executor malfunction.
	//
	// governance_unavailable stays distinct from refusal — one is an
	// infrastructure failure that must charge the circuit breaker, the other is
	// a decision that must not.
	var dispatchCheckID, execCheckID string
	if dr, ok := outputs["dispatch_result"].(map[string]any); ok {
		disposition, _ := dr["disposition"].(string)
		out.refused = disposition == engine.DispositionRefused
		dispatchCheckID, _ = dr["action_check_id"].(string)
		if st, ok := dr["status"].(string); ok {
			out.governanceStatus = st
		}
		if unavailable, ok := dr["governance_unavailable"].(bool); ok {
			out.governanceUnavailable = unavailable
		}
	}
	if exec, ok := outputs["execution_result"].(map[string]any); ok {
		out.executed, _ = exec["executed"].(bool)
		if id, ok := exec["audit_id"].(string); ok {
			out.auditID = id
		}
		// The executed path carries its authorizing decision here; the refusal
		// path never reaches this output at all.
		execCheckID, _ = exec["action_check_id"].(string)
		if ts, ok := exec["dispatched_at"].(string); ok && ts != "" {
			out.hasDispatchInstant = true
		}
	}

	// Two non-empty, DIFFERENT decision ids in one run is not a value to choose
	// between — it means the run contains two governance decisions for one
	// action, which is the exact duplication this architecture exists to
	// prevent. Picking either would report an attempt as authorized by a
	// decision that may not be the one that authorized it.
	if dispatchCheckID != "" && execCheckID != "" && dispatchCheckID != execCheckID {
		return out, fmt.Errorf("run reports conflicting action_check_ids (dispatch_result=%s "+
			"execution_result=%s) — refusing to attribute the attempt to either",
			dispatchCheckID, execCheckID)
	}
	out.actionCheckID = execCheckID
	if out.actionCheckID == "" {
		out.actionCheckID = dispatchCheckID
	}
	if v, ok := outputs["verification"].(map[string]any); ok {
		out.verified = true
		if converged, ok := v["converged"].(bool); ok {
			out.converged = converged
		}
	}
	return out, nil
}

// buildDoctorActorRouter creates a Router with the doctor's remediation
// action handlers wired to local state. This Router is used by the
// DoctorActorServer to handle callbacks from the workflow service.
func (s *ClusterDoctorServer) buildDoctorActorRouter() *engine.Router {
	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, s.buildDoctorRemediationConfig())
	return router
}

// buildDoctorRemediationConfig wires the five pipeline callbacks the
// workflow engine invokes via actor callbacks. All callbacks access
// in-process state (finding cache, ExecuteRemediation, collector).
func (s *ClusterDoctorServer) buildDoctorRemediationConfig() engine.DoctorRemediationConfig {
	return engine.DoctorRemediationConfig{
		ResolveFinding: func(ctx context.Context, runID, findingID string, stepIndex uint32, bindingMode string) (*engine.ResolvedFinding, error) {
			// The resolution contract is an explicit workflow input, never
			// inferred from whether a binding row happens to exist.
			//
			// Inferring it was unsafe: a binding can be absent because of
			// premature cleanup, context cancellation, an RPC timeout while
			// callbacks are still in flight, a mismatched run id, or a bug. Each
			// of those would have been read as "this must be an operator run"
			// and silently resolved a DIFFERENT finding from the mutable cache,
			// then executed a mutation the healer never authorized. Absence of
			// evidence is not evidence of a different caller.
			var f rules.Finding
			switch bindingMode {
			case engine.FindingBindingAutonomousRequired:
				bound, ok := s.runFindings.lookup(runID)
				if !ok {
					// Fail closed, including when a late callback arrives after
					// cleanup: the mode says a binding is REQUIRED, so a missing
					// one is an error about this run, never a licence to read
					// the cache.
					return nil, fmt.Errorf("resolve_finding: run %s declares %s but no run-scoped "+
						"finding binding exists — refusing to resolve from the mutable cache",
						runID, engine.FindingBindingAutonomousRequired)
				}
				if err := bound.matches(findingID, stepIndex); err != nil {
					return nil, fmt.Errorf("resolve_finding: run %s is bound to a different subject: %w "+
						"— refusing to resolve", runID, err)
				}
				f = bound.finding

			case engine.FindingBindingOperatorCurrent, "":
				// The two contracts are mutually exclusive. A run that HAS an
				// autonomous binding must never be resolved under the operator
				// contract, because that would read the mutable cache and could
				// substitute a different finding for one the healer bound.
				//
				// This is the downgrade path: a mode that is lost, corrupted, or
				// incorrectly propagated arrives here as operator_current or as
				// empty, and without this check it would silently demote an
				// autonomous run. Empty mode stays accepted for genuine legacy
				// operator callers, but never as a way past an existing binding.
				if _, bound := s.runFindings.lookup(runID); bound {
					return nil, fmt.Errorf("resolve_finding: run %s carries an autonomous finding "+
						"binding but requested mode %q — refusing to downgrade a bound run to the "+
						"operator resolution contract", runID, bindingMode)
				}
				// An operator named a finding id and expects the CURRENT
				// finding, so the cache is the right source here.
				s.lastFindingsMu.RLock()
				cached := make([]rules.Finding, len(s.lastFindings))
				copy(cached, s.lastFindings)
				s.lastFindingsMu.RUnlock()
				var ok bool
				f, ok = rules.FindByID(cached, findingID)
				if !ok {
					return nil, fmt.Errorf("finding %s not in last snapshot — call GetClusterReport first", findingID)
				}

			default:
				return nil, fmt.Errorf("resolve_finding: unknown finding_binding_mode %q — refusing to guess "+
					"which resolution contract applies", bindingMode)
			}
			steps := f.Remediation
			if int(stepIndex) >= len(steps) {
				return nil, fmt.Errorf("step_index %d out of range (finding has %d steps)", stepIndex, len(steps))
			}
			step := steps[stepIndex]
			action := step.GetAction()
			if action == nil {
				return &engine.ResolvedFinding{
					FindingID: findingID, StepIndex: stepIndex,
					HasAction:   false,
					Description: step.GetDescription(),
				}, nil
			}
			params := action.GetParams()
			// Subject identity comes from the FINDING and the local cluster,
			// never from the action params. EntityRef is taken from the finding
			// rather than defaulted to params["node_id"]: the two coincide for
			// node-scoped findings and diverge for service- or cluster-scoped
			// ones, and substituting one would attribute a later verification to
			// the wrong subject.
			//
			// Fail rather than invent. Downstream evidence is indexed by
			// cluster, so a remediation resolved without one would produce
			// verification that lands in the cluster-less partition and is
			// invisible to every cluster-scoped reader — worse than refusing,
			// because it looks recorded.
			if s.clusterID == "" {
				return nil, fmt.Errorf("resolve_finding: cluster id unknown — refusing to resolve %s "+
					"without cluster identity (verification evidence would be unattributable)", findingID)
			}
			if f.InvariantID == "" || f.EntityRef == "" {
				return nil, fmt.Errorf("resolve_finding: finding %s lacks identity (invariant_id=%q entity_ref=%q) — "+
					"refusing to resolve with incomplete lineage", findingID, f.InvariantID, f.EntityRef)
			}
			return &engine.ResolvedFinding{
				FindingID:   findingID,
				StepIndex:   stepIndex,
				NodeID:      params["node_id"],
				ActionType:  action.GetActionType().String(),
				Risk:        action.GetRisk().String(),
				Idempotent:  action.GetIdempotent(),
				Description: action.GetDescription(),
				HasAction:   true,
				ClusterID:   s.clusterID,
				InvariantID: f.InvariantID,
				EntityRef:   f.EntityRef,
			}, nil
		},

		ExecuteRemediation: func(ctx context.Context, runID, findingID string, stepIndex uint32,
			approvalToken string, dryRun bool, bindingMode string) (*engine.ExecutionResult, error) {

			req := &cluster_doctorpb.ExecuteRemediationRequest{
				FindingId:     findingID,
				StepIndex:     stepIndex,
				ApprovalToken: approvalToken,
				DryRun:        dryRun,
			}

			var resp *cluster_doctorpb.ExecuteRemediationResponse
			var err error
			switch bindingMode {
			case engine.FindingBindingAutonomousRequired:
				// Execute the BOUND finding by value.
				//
				// The public ExecuteRemediation RPC re-resolves the finding from
				// the mutable lastFindings cache. Calling it here would mean the
				// binding guarded resolution while the mutation was still free to
				// act on whatever the cache held by then — the protected half was
				// the half that does not touch the cluster.
				//
				// The ORIGINAL rules.Finding is used, not a reconstructed
				// ResolvedFinding: the executor needs the exact remediation action
				// and its parameters, and a rebuilt view could differ in precisely
				// the fields that decide what gets mutated.
				bound, ok := s.runFindings.lookup(runID)
				if !ok {
					return nil, fmt.Errorf("execute_remediation: run %s declares %s but no run-scoped "+
						"finding binding exists — refusing to mutate from the mutable cache",
						runID, engine.FindingBindingAutonomousRequired)
				}
				if mErr := bound.matches(findingID, stepIndex); mErr != nil {
					return nil, fmt.Errorf("execute_remediation: run %s is bound to a different subject: %w "+
						"— refusing to mutate", runID, mErr)
				}
				if s.executeForFindingHook != nil {
					resp, err = s.executeForFindingHook(bound.finding)
				} else {
					resp, err = s.executeRemediationForFinding(ctx, bound.finding, req)
				}

			case engine.FindingBindingOperatorCurrent, "":
				// Mutually exclusive with the autonomous contract, on the same
				// terms as resolution: a bound run must never be executed through
				// the current-finding path.
				if _, bound := s.runFindings.lookup(runID); bound {
					return nil, fmt.Errorf("execute_remediation: run %s carries an autonomous finding "+
						"binding but requested mode %q — refusing to downgrade a bound run",
						runID, bindingMode)
				}
				resp, err = s.ExecuteRemediation(ctx, req)

			default:
				return nil, fmt.Errorf("execute_remediation: unknown finding_binding_mode %q — "+
					"refusing to guess which resolution contract applies", bindingMode)
			}
			if err != nil {
				return nil, err
			}
			return &engine.ExecutionResult{
				AuditID:  resp.GetAuditId(),
				Status:   resp.GetStatus(),
				Executed: resp.GetExecuted(),
				Output:   resp.GetOutput(),
				Reason:   resp.GetReason(),
			}, nil
		},

		VerifyConvergence: func(ctx context.Context, findingID, nodeID string, dispatchedAt time.Time) (*engine.Verification, error) {
			// Post-action verification must read state collected AFTER the repair.
			//
			// This previously called collector.GetSnapshot, which serves the cache
			// and could therefore return the very pre-repair snapshot the finding
			// was raised from. The finding is still present in that data, so a
			// repair that worked verifies as "still present" — the workflow then
			// records a failed outcome and the learning loop accumulates support
			// for the proposition that the repair does not work.
			//
			// Forcing fresh is necessary but NOT sufficient: the collector can join
			// an already in-flight fetch that STARTED BEFORE the dispatch, which
			// returns CacheHit=false while still describing pre-repair state. So
			// freshness is proven by ordering, not by the cache flag — GeneratedAt
			// must strictly post-date the dispatch instant.
			//
			// takeSnapshot carries the leader-only force-fresh rule. A follower
			// serves cached, the ordering check rejects it, and the step fails
			// rather than recording a guess.
			if dispatchedAt.IsZero() {
				return nil, fmt.Errorf("verify_convergence: no dispatch timestamp — " +
					"cannot prove evidence was collected after the action; refusing to verify")
			}
			snap, _, err := s.takeSnapshot(ctx, cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH)
			if err != nil && snap == nil {
				return nil, fmt.Errorf("verify snapshot fetch: %w", err)
			}
			if !verificationSnapshotIsPostAction(snap, dispatchedAt) {
				return nil, fmt.Errorf("verify_convergence: no provably post-action snapshot "+
					"(dispatched_at=%s snapshot_generated_at=%s) — refusing to verify against "+
					"evidence that may predate the repair",
					dispatchedAt.UTC().Format(time.RFC3339Nano),
					snapGeneratedAt(snap).UTC().Format(time.RFC3339Nano))
			}
			var findings []rules.Finding
			clusterWide := nodeID == ""
			if clusterWide {
				findings = s.registry.EvaluateAll(snap)
			} else {
				findings = s.registry.EvaluateForNode(snap, nodeID)
			}
			// Only cluster-wide evaluations are authoritative for the
			// cluster.finding.* event delta.
			s.cacheFindings(findings, clusterWide)

			stillPresent := false
			for _, f := range findings {
				if f.FindingID == findingID {
					stillPresent = true
					break
				}
			}
			return &engine.Verification{
				Converged:           !stillPresent,
				FindingStillPresent: stillPresent,
				RemainingRelated:    0,
			}, nil
		},

		// Close the learning loop. The doctor observed the finding, the workflow
		// acted, and this records whether the action worked. Supplied here
		// rather than inside the engine so the workflow engine stays free of
		// behavioral-memory types and this service keeps ownership of what it
		// records — it already owns the bounded recorder for the finding path.
		ObserveOutcome: func(ctx context.Context, o remediation.Outcome) {
			// The single producer of remediation outcomes, for operator-started
			// and autonomous runs alike. The healer no longer records its own:
			// one executed action now yields exactly one behavioral outcome, one
			// governed-outcome link, and at most one support contribution.
			s.observeOutcome(ctx, o, o.ActionCheckID)
		},

		// The governed gate, consulted immediately before dispatch.
		GateAction: s.gateRemediation,

		MarkFailed: func(ctx context.Context, findingID string) error {
			slog.Warn("remediate.doctor.finding workflow failed",
				"finding_id", findingID,
			)
			return nil
		},
	}
}
