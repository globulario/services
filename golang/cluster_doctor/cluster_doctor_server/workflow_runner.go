package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
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

	// The doctor's own gRPC address is the callback endpoint for the
	// workflow service to dispatch doctor actions back to.
	doctorEndpoint := fmt.Sprintf("localhost:%d", s.cfg.Port)

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
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return rules.DispatchResult{}, fmt.Errorf("marshal inputs: %w", err)
	}

	resp, err := s.workflowClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    s.clusterID,
		WorkflowName: remediationWorkflowName,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-doctor": fmt.Sprintf("localhost:%d", s.cfg.Port),
		},
		RunId:         runID,
		CorrelationId: correlationID,
	})
	if err != nil {
		return rules.DispatchResult{WorkflowRunID: runID},
			fmt.Errorf("start remediation workflow: %w", err)
	}

	return classifyRemediationRun(runID, resp, dryRun), nil
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

	outcome := remediationOutcomeFromRun(resp)
	res.AuditID = outcome.auditID
	res.Executed = outcome.executed
	res.Verified = outcome.verified
	res.Converged = outcome.converged

	switch {
	case outcome.refused:
		// Governance declined before any side effect. Not a malfunction.
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
}

// remediationOutcomeFromRun reads the canonical remediation_outcome the verify
// step emitted into the run's outputs. Absent outputs leave every field false,
// which classifies as a failure rather than a success — the safe direction when
// a run's own account of itself is missing.
func remediationOutcomeFromRun(resp *workflowpb.ExecuteWorkflowResponse) runRemediationOutcome {
	var out runRemediationOutcome
	if resp == nil || resp.GetOutputsJson() == "" {
		return out
	}
	var outputs map[string]any
	if err := json.Unmarshal([]byte(resp.GetOutputsJson()), &outputs); err != nil {
		return out
	}
	if exec, ok := outputs["execution_result"].(map[string]any); ok {
		out.executed, _ = exec["executed"].(bool)
		if id, ok := exec["audit_id"].(string); ok {
			out.auditID = id
		}
		// A governed refusal is a structured state on the execute step, not an
		// error string.
		if refused, ok := exec["governance_refused"].(bool); ok && refused {
			out.refused = true
		}
	}
	if v, ok := outputs["verification"].(map[string]any); ok {
		out.verified = true
		if converged, ok := v["converged"].(bool); ok {
			out.converged = converged
		}
	}
	return out
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
		ResolveFinding: func(ctx context.Context, runID, findingID string, stepIndex uint32) (*engine.ResolvedFinding, error) {
			// Autonomous runs resolve from the binding written before the run
			// started, never from lastFindings.
			//
			// The healer selected an exact finding during its enforcement cycle
			// and the cache has been republished at least once since. Re-reading
			// it here could return a different finding under the same id, and the
			// workflow would then execute a mutation against a subject the healer
			// never authorized. A bound run therefore demands ITS finding, and a
			// mismatch fails closed rather than falling back.
			//
			// Absence of a binding means an operator started this run by naming a
			// finding id, which is a different contract: the operator wants the
			// current finding, so the cache lookup below is correct for that path.
			var f rules.Finding
			if bound, isAutonomous := s.runFindings.lookup(runID); isAutonomous {
				if err := bound.matches(findingID, stepIndex); err != nil {
					return nil, fmt.Errorf("resolve_finding: run %s is bound to a different subject: %w "+
						"— refusing to resolve", runID, err)
				}
				f = bound.finding
			} else {
				s.lastFindingsMu.RLock()
				cached := make([]rules.Finding, len(s.lastFindings))
				copy(cached, s.lastFindings)
				s.lastFindingsMu.RUnlock()
				var ok bool
				f, ok = rules.FindByID(cached, findingID)
				if !ok {
					return nil, fmt.Errorf("finding %s not in last snapshot — call GetClusterReport first", findingID)
				}
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

		ExecuteRemediation: func(ctx context.Context, findingID string, stepIndex uint32, approvalToken string, dryRun bool) (*engine.ExecutionResult, error) {
			resp, err := s.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{
				FindingId:     findingID,
				StepIndex:     stepIndex,
				ApprovalToken: approvalToken,
				DryRun:        dryRun,
			})
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
