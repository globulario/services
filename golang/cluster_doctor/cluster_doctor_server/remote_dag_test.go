package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/actortransport"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// Full remote DAG proof.
//
//	real remediate.doctor.finding.yaml
//	  -> real engine.Engine
//	    -> PRODUCTION remote handler (actortransport.Handler, the same code
//	       workflow_server.actorDispatcher delegates to)
//	      -> real gRPC over bufconn
//	        -> real DoctorActorServer
//	          -> real doctor remediation handlers
//	            -> engine merge
//	              -> terminal run outputs
//
// Nothing here reimplements transport. Every earlier "real execution" test used
// an in-process router, where ActionRequest.Outputs is the run's live map — so a
// handler that mutated it appeared to work while the deployed topology, which
// serializes that map and throws the copy away, silently lost the write. Two
// rounds of P1 defects lived in precisely that gap.

// remoteHarness wires the whole chain and records what each callback saw.
type remoteHarness struct {
	srv *ClusterDoctorServer

	gate     int
	exec     int
	verify   int
	observe  int
	dryRuns  []bool
	executed []rules.Finding
	outcomes []remediation.Outcome

	// scripted behavior
	verifiedAt   []time.Time // dispatchedAt values the verifier actually received
	verdict      *engine.GateVerdict
	gateErr      error
	converged    bool
	verifyErr    error
	beforeExec   func() // runs between resolution and execution
	publicExecOK bool   // set if the PUBLIC ExecuteRemediation path is ever used
}

const (
	rdRun     = "run-remote-dag"
	rdCluster = "c1"
)

func newRemoteHarness(t *testing.T) *remoteHarness {
	t.Helper()
	h := &remoteHarness{converged: true}
	h.verdict = &engine.GateVerdict{
		ActionCheckID: "chk-remote-1", Governed: true, Allowed: true, Status: "allowed",
	}

	srv := &ClusterDoctorServer{
		clusterID:             rdCluster,
		cfg:                   &clusterdoctorConfig{Port: 10300},
		actorEndpointResolver: func() string { return "10.0.0.63:10300" },
	}
	srv.isAuthoritative.Store(true)

	// Observe which finding actually reaches the mutation.
	srv.executeForFindingHook = func(f rules.Finding) (*cluster_doctorpb.ExecuteRemediationResponse, error) {
		h.exec++
		h.executed = append(h.executed, f)
		return &cluster_doctorpb.ExecuteRemediationResponse{
			AuditId: "rem-remote-1", Executed: true, Status: "executed",
		}, nil
	}
	h.srv = srv
	return h
}

// config builds the doctor's real remediation config, overriding only the
// collector-backed verifier and the governor — the two things a test cannot
// stand up infra-free. Resolution, execution routing, binding enforcement and
// outcome recording are the PRODUCTION implementations.
func (h *remoteHarness) config(t *testing.T) engine.DoctorRemediationConfig {
	t.Helper()
	cfg := h.srv.buildDoctorRemediationConfig()

	prodExec := cfg.ExecuteRemediation
	cfg.ExecuteRemediation = func(ctx context.Context, runID, findingID string, stepIndex uint32,
		approvalToken string, dryRun bool, bindingMode string) (*engine.ExecutionResult, error) {
		if h.beforeExec != nil {
			h.beforeExec()
			h.beforeExec = nil
		}
		h.dryRuns = append(h.dryRuns, dryRun)
		return prodExec(ctx, runID, findingID, stepIndex, approvalToken, dryRun, bindingMode)
	}

	cfg.GateAction = func(context.Context, engine.GateRequest) (engine.GateVerdict, error) {
		h.gate++
		if h.gateErr != nil {
			return engine.GateVerdict{}, h.gateErr
		}
		return *h.verdict, nil
	}
	cfg.VerifyConvergence = func(_ context.Context, _, _ string, dispatchedAt time.Time) (*engine.Verification, error) {
		h.verify++
		// Captured, not ignored. The production verifier proves its snapshot
		// post-dates this instant; a test verifier that discards the argument
		// would still pass if the timestamp vanished in transport, which is
		// half of the original lineage defect.
		h.verifiedAt = append(h.verifiedAt, dispatchedAt)
		if h.verifyErr != nil {
			return nil, h.verifyErr
		}
		return &engine.Verification{Converged: h.converged, FindingStillPresent: !h.converged}, nil
	}
	cfg.ObserveOutcome = func(_ context.Context, o remediation.Outcome) {
		h.observe++
		h.outcomes = append(h.outcomes, o)
	}
	return cfg
}

// run executes the canonical definition end to end over the real transport.
func (h *remoteHarness) run(t *testing.T, inputs map[string]any) *engine.Run {
	t.Helper()

	// Actor side: real server, real handlers, real gRPC.
	actorRouter := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(actorRouter, h.config(t))
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(gs, NewDoctorActorServer(actorRouter))
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Engine side: the PRODUCTION remote handler as the cluster-doctor fallback.
	engineRouter := engine.NewRouter()
	engineRouter.RegisterFallback(v1alpha1.ActorClusterDoctor,
		actortransport.Handler(string(v1alpha1.ActorClusterDoctor),
			actortransport.StaticClient(workflowpb.NewWorkflowActorServiceClient(conn))))

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../../workflow/definitions/remediate.doctor.finding.yaml")
	if err != nil {
		t.Fatalf("load canonical definition: %v", err)
	}

	eng := &engine.Engine{Router: engineRouter, RunID: rdRun}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	run, _ := eng.Execute(ctx, def, inputs)
	if run == nil {
		t.Fatal("engine returned a nil run")
	}
	return run
}

// classify projects the real run into the response envelope the classifier
// consumes.
//
// The ENVELOPE is a test projection; the OUTPUTS are not. Run id, status and
// outputs all come from the real engine run, so the classifier is judging what
// the transport actually produced.
func (h *remoteHarness) classify(t *testing.T, run *engine.Run, dryRun bool) rules.DispatchResult {
	t.Helper()
	outJSON, err := json.Marshal(run.Outputs)
	if err != nil {
		t.Fatalf("serialize real run outputs: %v", err)
	}
	status := "FAILED"
	if run.Status == engine.RunSucceeded {
		status = "SUCCEEDED"
	}
	return classifyRemediationRun(rdRun, &workflowpb.ExecuteWorkflowResponse{
		RunId: rdRun, Status: status, OutputsJson: string(outJSON),
	}, dryRun)
}

func rdInputs(mode string, dryRun bool) map[string]any {
	return map[string]any{
		"finding_id":           "f-auto-1",
		"step_index":           0,
		"dry_run":              dryRun,
		"approval_token":       "",
		"finding_binding_mode": mode,
	}
}

func (h *remoteHarness) bind(t *testing.T, f rules.Finding) {
	t.Helper()
	if err := h.srv.runFindings.bind(rdRun, f, 0); err != nil {
		t.Fatalf("bind: %v", err)
	}
	t.Cleanup(func() { h.srv.runFindings.release(rdRun) })
}

// ── allowed and converged ──────────────────────────────────────────────────

func TestRemoteDAG_AllowedAndConverged(t *testing.T) {
	h := newRemoteHarness(t)
	f := autonomousFinding()
	h.bind(t, f)

	run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))

	if run.Status != engine.RunSucceeded {
		t.Fatalf("run status = %s, want SUCCEEDED (outputs=%v)", run.Status, outputKeysOf(run.Outputs))
	}
	// The named resolve delta crossed the wire and reached the later steps.
	if _, ok := run.Outputs["resolved_finding"].(map[string]any); !ok {
		t.Errorf("resolved_finding missing from run outputs — assess_risk could not have read " +
			"$.resolved_finding, which is where the remote run used to die")
	}
	if h.gate != 1 {
		t.Errorf("governance called %d time(s), want exactly 1", h.gate)
	}
	if h.exec != 1 {
		t.Errorf("executor called %d time(s), want exactly 1", h.exec)
	}
	if h.verify != 1 {
		t.Errorf("verifier called %d time(s), want exactly 1", h.verify)
	}
	if h.observe != 1 {
		t.Errorf("observer called %d time(s), want exactly 1", h.observe)
	}
	if len(h.executed) == 1 && h.executed[0].EntityRef != f.EntityRef {
		t.Errorf("executor received %q, want the bound finding %q", h.executed[0].EntityRef, f.EntityRef)
	}
	// Lineage must be COMPLETE end to end, not merely present in parts.
	if len(h.verifiedAt) != 1 {
		t.Fatalf("verifier received %d dispatch timestamps, want 1", len(h.verifiedAt))
	}
	dispatchedAt := h.verifiedAt[0]
	if dispatchedAt.IsZero() {
		t.Error("verifier received a ZERO dispatch timestamp — the execute step's dispatched_at " +
			"did not survive the actor RPC, so post-action ordering cannot be proven")
	}
	if len(h.outcomes) == 1 {
		o := h.outcomes[0]
		if o.WorkflowRunID != rdRun {
			t.Errorf("outcome WorkflowRunID = %q, want the bound run %q", o.WorkflowRunID, rdRun)
		}
		if o.ActionCheckID != "chk-remote-1" {
			t.Errorf("outcome ActionCheckID = %q, want chk-remote-1", o.ActionCheckID)
		}
		// The SAME instant the verifier was judged against must be the one the
		// outcome records. Two readings that could drift would let a
		// verification be judged against one timestamp and reported with
		// another.
		if !o.DispatchedAt.Equal(dispatchedAt) {
			t.Errorf("outcome DispatchedAt = %s, want the verifier's %s — one dispatch instant, "+
				"read once", o.DispatchedAt, dispatchedAt)
		}
		if !o.LineageComplete() {
			t.Errorf("a successful autonomous repair must be lineage-complete; defects=%v",
				o.LineageDefects())
		}
	}
	if ro, ok := run.Outputs["remediation_outcome"].(map[string]any); ok {
		if complete, _ := ro["lineage_complete"].(bool); !complete {
			t.Error("remediation_outcome.lineage_complete must be true in the terminal run outputs — " +
				"this is the receipt an out-of-process consumer reads")
		}
		if ts, _ := ro["dispatched_at"].(string); ts == "" {
			t.Error("remediation_outcome.dispatched_at must survive into run outputs")
		}
	} else {
		t.Error("remediation_outcome missing from terminal run outputs")
	}

	res := h.classify(t, run, false)
	if res.Disposition != rules.DispatchConverged {
		t.Errorf("disposition = %q, want %q", res.Disposition, rules.DispatchConverged)
	}
	report := healReportFor(t, res)
	if report.AutoFixed != 1 {
		t.Errorf("AutoFixed = %d, want 1", report.AutoFixed)
	}
}

// ── bound identity despite cache replacement ───────────────────────────────

func TestRemoteDAG_BoundIdentitySurvivesCacheReplacement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		replace func(srv *ClusterDoctorServer)
	}{
		{"cache emptied", func(srv *ClusterDoctorServer) {
			srv.lastFindingsMu.Lock()
			srv.lastFindings = nil
			srv.lastFindingsMu.Unlock()
		}},
		{"same id, different subject", func(srv *ClusterDoctorServer) {
			impostor := autonomousFinding()
			impostor.EntityRef = "node-9/globular-attacker.service"
			impostor.Remediation[0].Action.Params = map[string]string{
				"node_id": "node-9", "unit": "globular-attacker.service",
			}
			srv.lastFindingsMu.Lock()
			srv.lastFindings = []rules.Finding{impostor}
			srv.lastFindingsMu.Unlock()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRemoteHarness(t)
			f := autonomousFinding()
			h.srv.lastFindings = []rules.Finding{f}
			h.bind(t, f)

			// Mutate the cache AFTER resolution, immediately before execution.
			h.beforeExec = func() { tc.replace(h.srv) }

			run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))
			if run.Status != engine.RunSucceeded {
				t.Fatalf("run must still succeed; status=%s", run.Status)
			}
			if h.exec != 1 {
				t.Fatalf("executor called %d time(s), want exactly 1", h.exec)
			}
			got := h.executed[0]
			if got.EntityRef != f.EntityRef {
				t.Errorf("executed subject = %q, want the BOUND %q — the cache substituted a "+
					"different finding under the same id and the mutation followed it",
					got.EntityRef, f.EntityRef)
			}
			p := got.Remediation[0].GetAction().GetParams()
			if p["node_id"] != "node-1" || p["unit"] != "globular-log.service" {
				t.Errorf("executed action params = %v, want the bound finding's action", p)
			}
		})
	}
}

// ── missing / mismatched binding ───────────────────────────────────────────

func TestRemoteDAG_BindingFailuresStopBeforeExecutor(t *testing.T) {
	f := autonomousFinding()

	for _, tc := range []struct {
		name string
		bind bool
		mode string
	}{
		{"no binding", false, engine.FindingBindingAutonomousRequired},
		{"mismatched finding id", true, engine.FindingBindingAutonomousRequired},
		{"mismatched step index", true, engine.FindingBindingAutonomousRequired},
		{"operator mode on a bound run", true, engine.FindingBindingOperatorCurrent},
		{"empty mode on a bound run", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRemoteHarness(t)
			// The cache COULD satisfy a fallback — that is what makes these
			// assertions non-vacuous.
			h.srv.lastFindings = []rules.Finding{f}
			if tc.bind {
				h.bind(t, f)
			}

			inputs := rdInputs(tc.mode, false)
			switch tc.name {
			case "mismatched finding id":
				inputs["finding_id"] = "f-different"
			case "mismatched step index":
				inputs["step_index"] = 7
			}
			run := h.run(t, inputs)
			if run.Status == engine.RunSucceeded {
				t.Fatal("a binding failure must not produce a succeeded run")
			}
			if h.exec != 0 {
				t.Errorf("executor called %d time(s), want 0 — the mutation must be stopped "+
					"before it happens, not reported afterwards", h.exec)
			}
		})
	}
}

// ── governed refusal ───────────────────────────────────────────────────────

func TestRemoteDAG_GovernedRefusal(t *testing.T) {
	h := newRemoteHarness(t)
	f := autonomousFinding()
	h.bind(t, f)
	h.verdict = &engine.GateVerdict{
		ActionCheckID: "chk-refused-remote", Governed: true, Allowed: false,
		Status: "needs_evidence", Reason: "required evidence does not qualify",
	}

	run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))
	if run.Status == engine.RunSucceeded {
		t.Fatal("a governed refusal must fail the run")
	}

	dr, ok := run.Outputs["dispatch_result"].(map[string]any)
	if !ok {
		t.Fatalf("dispatch_result missing from the REAL run outputs (have %v) — the receipt did "+
			"not survive the actor RPC, so a refusal is indistinguishable from an executor failure",
			outputKeysOf(run.Outputs))
	}
	if dr["disposition"] != engine.DispositionRefused {
		t.Errorf("disposition = %v, want %q", dr["disposition"], engine.DispositionRefused)
	}
	if dr["action_check_id"] != "chk-refused-remote" {
		t.Errorf("action_check_id = %v, want chk-refused-remote", dr["action_check_id"])
	}
	if dr["status"] != "needs_evidence" {
		t.Errorf("governance status = %v, want needs_evidence", dr["status"])
	}
	if h.exec != 0 || h.verify != 0 || h.observe != 0 {
		t.Errorf("refusal touched something: exec=%d verify=%d observe=%d", h.exec, h.verify, h.observe)
	}

	res := h.classify(t, run, false)
	if res.Disposition != rules.DispatchRefused {
		t.Fatalf("disposition = %q, want %q", res.Disposition, rules.DispatchRefused)
	}
	report := healReportFor(t, res)
	if report.Refused != 1 {
		t.Errorf("Refused = %d, want 1", report.Refused)
	}
	if report.Errors != 0 {
		t.Errorf("Errors = %d, want 0 — a refusal is not a failed mutation", report.Errors)
	}
}

// ── governance unavailable ─────────────────────────────────────────────────

func TestRemoteDAG_GovernanceUnavailable(t *testing.T) {
	h := newRemoteHarness(t)
	h.bind(t, autonomousFinding())
	h.gateErr = fmt.Errorf("behavioral memory unreachable")

	run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))

	dr, ok := run.Outputs["dispatch_result"].(map[string]any)
	if !ok {
		t.Fatalf("dispatch_result missing from real run outputs (have %v)", outputKeysOf(run.Outputs))
	}
	if unavailable, _ := dr["governance_unavailable"].(bool); !unavailable {
		t.Error("governance_unavailable must survive so an outage stays distinguishable from refusals")
	}
	if h.exec != 0 || h.observe != 0 {
		t.Errorf("nothing may run: exec=%d observe=%d", h.exec, h.observe)
	}

	res := h.classify(t, run, false)
	if res.Disposition != rules.DispatchExecutionFailed {
		t.Fatalf("disposition = %q, want %q", res.Disposition, rules.DispatchExecutionFailed)
	}
	report := healReportFor(t, res)
	if report.Errors != 1 || report.ExecutionFailed != 1 {
		t.Errorf("Errors=%d ExecutionFailed=%d, want 1/1 — an outage charges the budget",
			report.Errors, report.ExecutionFailed)
	}
}

// ── verification failure, both shapes ──────────────────────────────────────

func TestRemoteDAG_VerificationFailures(t *testing.T) {
	t.Run("no qualifying post-action snapshot", func(t *testing.T) {
		h := newRemoteHarness(t)
		h.bind(t, autonomousFinding())
		h.verifyErr = fmt.Errorf("no provably post-action snapshot available")

		run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))
		if h.exec != 1 {
			t.Fatalf("executor called %d time(s), want 1 — the mutation DID happen", h.exec)
		}
		if _, ok := run.Outputs["execution_result"].(map[string]any); !ok {
			t.Error("the execution receipt must survive an unverifiable run")
		}
		res := h.classify(t, run, false)
		if res.Disposition != rules.DispatchExecutedUnverified {
			t.Errorf("disposition = %q, want %q — convergence is UNKNOWN and must not be claimed",
				res.Disposition, rules.DispatchExecutedUnverified)
		}
		report := healReportFor(t, res)
		if report.AutoFixed != 0 {
			t.Errorf("AutoFixed = %d, want 0", report.AutoFixed)
		}
	})

	t.Run("verifier ran and finding remains", func(t *testing.T) {
		h := newRemoteHarness(t)
		h.bind(t, autonomousFinding())
		h.converged = false

		run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))
		if h.observe != 1 {
			t.Errorf("observer called %d time(s), want exactly 1 — a failed repair is the outcome "+
				"a promotion decision most needs", h.observe)
		}
		if len(h.outcomes) == 1 && h.outcomes[0].FindingResolved {
			t.Error("outcome must record FindingResolved=false")
		}
		res := h.classify(t, run, false)
		if res.Disposition != rules.DispatchExecutedNotConverged {
			t.Errorf("disposition = %q, want %q", res.Disposition, rules.DispatchExecutedNotConverged)
		}
		report := healReportFor(t, res)
		if report.ExecutedNotConverged != 1 || report.Errors != 1 {
			t.Errorf("ExecutedNotConverged=%d Errors=%d, want 1/1",
				report.ExecutedNotConverged, report.Errors)
		}
	})
}

// ── dry run ────────────────────────────────────────────────────────────────

func TestRemoteDAG_DryRun(t *testing.T) {
	h := newRemoteHarness(t)
	h.bind(t, autonomousFinding())
	h.srv.executeForFindingHook = func(f rules.Finding) (*cluster_doctorpb.ExecuteRemediationResponse, error) {
		h.exec++
		h.executed = append(h.executed, f)
		// A rehearsal executes nothing.
		return &cluster_doctorpb.ExecuteRemediationResponse{
			AuditId: "rem-dry", Executed: false, Status: "dry_run",
		}, nil
	}

	run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, true))

	if len(h.dryRuns) != 1 || !h.dryRuns[0] {
		t.Errorf("executor received dryRun=%v, want a single true", h.dryRuns)
	}
	for _, o := range h.outcomes {
		if o.Dispatched {
			t.Error("a dry run must not record an outcome claiming dispatch")
		}
	}
	if h.verify != 0 {
		t.Errorf("verifier called %d time(s) on a dry run, want 0 — the definition skips "+
			"verification for a rehearsal, and nothing was mutated to verify", h.verify)
	}
	if h.observe != 0 {
		t.Errorf("observer called %d time(s) on a dry run, want 0 — counting a rehearsal would "+
			"manufacture promotion support for an action nobody performed", h.observe)
	}
	res := h.classify(t, run, true)
	if res.Disposition != rules.DispatchProposed {
		t.Errorf("disposition = %q, want %q", res.Disposition, rules.DispatchProposed)
	}
	report := healReportFor(t, res)
	if report.AutoFixed != 0 || report.Proposed != 1 {
		t.Errorf("AutoFixed=%d Proposed=%d, want 0/1", report.AutoFixed, report.Proposed)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

// healReportFor runs one dispatch result through the real Healer classification
// so counter semantics are proven end to end rather than asserted by hand.
func healReportFor(t *testing.T, res rules.DispatchResult) rules.HealReport {
	t.Helper()
	h := &rules.Healer{
		Dispatcher: &fixedDispatcher{res: res},
		PolicyLookup: func(string) rules.HealRule {
			return rules.HealRule{
				InvariantID: "node.systemd.units_running",
				Disposition: rules.HealAuto,
				AutoAction:  "restart_drifted_unit",
			}
		},
	}
	return h.Evaluate(context.Background(), []rules.Finding{{
		FindingID:       "f-auto-1",
		InvariantID:     "node.systemd.units_running",
		EntityRef:       "node-1/globular-log.service",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}})
}

type fixedDispatcher struct{ res rules.DispatchResult }

func (d *fixedDispatcher) Dispatch(context.Context, rules.Finding, string, bool) rules.DispatchResult {
	return d.res
}

func outputKeysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestRemoteDAG_MissingDispatchedAtIsDetected is the negative control for
// lineage continuity.
//
// If dispatched_at ever stops surviving the actor RPC, the verifier receives a
// zero instant and the outcome loses the binding that places verification after
// the action. That is half of the original lineage defect, and it would be
// invisible to a test whose verifier ignores the argument — which is exactly
// what this suite did until now.
func TestRemoteDAG_MissingDispatchedAtIsDetected(t *testing.T) {
	h := newRemoteHarness(t)
	h.bind(t, autonomousFinding())
	// Simulate the timestamp vanishing: the executor reports success but the
	// step records no dispatch instant.
	h.srv.executeForFindingHook = func(f rules.Finding) (*cluster_doctorpb.ExecuteRemediationResponse, error) {
		h.exec++
		h.executed = append(h.executed, f)
		return &cluster_doctorpb.ExecuteRemediationResponse{
			AuditId: "rem-no-ts", Executed: false, Status: "executed",
		}, nil
	}

	run := h.run(t, rdInputs(engine.FindingBindingAutonomousRequired, false))

	// With nothing dispatched, no dispatch instant exists, so the run must NOT
	// present itself as a lineage-complete success.
	if ro, ok := run.Outputs["remediation_outcome"].(map[string]any); ok {
		if complete, _ := ro["lineage_complete"].(bool); complete {
			t.Error("a run with no dispatch instant must not report lineage_complete")
		}
	}
	res := h.classify(t, run, false)
	if res.Disposition == rules.DispatchConverged {
		t.Error("a run that never dispatched must not classify as CONVERGED")
	}
}
