package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// Transport-boundary proof.
//
// The earlier "real execution" tests registered handlers into an IN-PROCESS
// router, where ActionRequest.Outputs is the run's live map shared by
// reference. A handler writing req.Outputs["dispatch_result"] therefore
// appeared to work.
//
// Production crosses WorkflowActorService over JSON/gRPC. There, Outputs is a
// deserialized copy that dies with the call, and ExecuteAction dropped
// OutputJson entirely whenever the handler failed — so a governed refusal
// arrived as a bare failure and charged the healer's circuit breaker.
//
// These tests drive the REAL DoctorActorServer over a real gRPC connection, so
// only what actually crosses the wire can satisfy them.

// startDoctorActor serves the real DoctorActorServer over bufconn and returns a
// client. Infra-free: no listener on a real port, no TLS, no cluster.
func startDoctorActor(t *testing.T, cfg engine.DoctorRemediationConfig) workflowpb.WorkflowActorServiceClient {
	t.Helper()
	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, cfg)

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(gs, NewDoctorActorServer(router))
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return workflowpb.NewWorkflowActorServiceClient(conn)
}

// callAction invokes one action across the wire and returns the response plus
// its decoded output delta.
func callAction(t *testing.T, c workflowpb.WorkflowActorServiceClient,
	action string, with, outputs map[string]any) (*workflowpb.ExecuteActionResponse, map[string]any) {
	t.Helper()
	withJSON, _ := json.Marshal(with)
	outJSON, _ := json.Marshal(outputs)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.ExecuteAction(ctx, &workflowpb.ExecuteActionRequest{
		RunId: "run-transport", StepId: action, Actor: "cluster-doctor", Action: action,
		WithJson: string(withJSON), OutputsJson: string(outJSON),
	})
	if err != nil {
		t.Fatalf("%s over transport: %v", action, err)
	}
	var delta map[string]any
	if resp.GetOutputJson() != "" {
		if err := json.Unmarshal([]byte(resp.GetOutputJson()), &delta); err != nil {
			t.Fatalf("decode output delta: %v", err)
		}
	}
	return resp, delta
}

func transportConfig(t *testing.T) (engine.DoctorRemediationConfig, *transportSpy) {
	t.Helper()
	spy := &transportSpy{}
	cfg := engine.DoctorRemediationConfig{
		ResolveFinding: func(_ context.Context, _, id string, idx uint32, _ string) (*engine.ResolvedFinding, error) {
			spy.resolve++
			return &engine.ResolvedFinding{
				FindingID: id, StepIndex: idx, NodeID: "node-1", HasAction: true,
				ActionType: "SYSTEMCTL_RESTART", Risk: "RISK_LOW",
				ClusterID: "c1", InvariantID: "node.systemd.units_running",
				EntityRef: "node-1/globular-log.service",
			}, nil
		},
		ExecuteRemediation: func(context.Context, string, string, uint32, string, bool, string) (*engine.ExecutionResult, error) {
			spy.exec++
			return &engine.ExecutionResult{AuditID: "rem-1", Executed: true, Status: "executed"}, nil
		},
		VerifyConvergence: func(context.Context, string, string, time.Time) (*engine.Verification, error) {
			spy.verify++
			return &engine.Verification{Converged: true}, nil
		},
		ObserveOutcome: func(_ context.Context, o remediation.Outcome) {
			spy.observe++
			spy.outcomes = append(spy.outcomes, o)
		},
		GateAction: func(context.Context, engine.GateRequest) (engine.GateVerdict, error) {
			spy.gate++
			return engine.GateVerdict{ActionCheckID: "chk-1", Governed: true, Allowed: true, Status: "allowed"}, nil
		},
	}
	return cfg, spy
}

type transportSpy struct {
	resolve, exec, verify, observe, gate int
	outcomes                             []remediation.Outcome
}

// TestActorTransport_ResolveReturnsNamedOutput proves finding 1: the named
// wrapper survives the RPC, so the next step's $.resolved_finding is defined.
//
// Before this, the handler returned bare finding fields. In-process the write
// to req.Outputs covered it up; remotely the engine merged the bare fields at
// the run-output ROOT and the workflow died at assess_risk — before governance
// or remediation ever ran.
func TestActorTransport_ResolveReturnsNamedOutput(t *testing.T) {
	cfg, _ := transportConfig(t)
	c := startDoctorActor(t, cfg)

	resp, delta := callAction(t, c, "doctor.resolve_finding",
		map[string]any{"finding_id": "f-1", "step_index": 0,
			"finding_binding_mode": engine.FindingBindingOperatorCurrent},
		map[string]any{})

	if !resp.GetOk() {
		t.Fatalf("resolve must succeed; message=%s", resp.GetMessage())
	}
	rf, ok := delta["resolved_finding"].(map[string]any)
	if !ok {
		t.Fatalf("OutputJson must carry a named resolved_finding delta; got %v — bare fields "+
			"merge at the run-output root and leave $.resolved_finding undefined", delta)
	}
	if rf["cluster_id"] != "c1" || rf["entity_ref"] != "node-1/globular-log.service" {
		t.Errorf("resolved_finding lost subject identity across the wire: %v", rf)
	}
	if _, leaked := delta["cluster_id"]; leaked {
		t.Error("the delta must not also publish bare fields at the root — downstream steps " +
			"name these outputs, and duplicates invite the two shapes to drift")
	}
}

// TestActorTransport_RefusalReceiptSurvivesFailedCallback proves finding 2, the
// defect that made the entire refusal contract unreachable in production.
func TestActorTransport_RefusalReceiptSurvivesFailedCallback(t *testing.T) {
	cfg, spy := transportConfig(t)
	cfg.GateAction = func(context.Context, engine.GateRequest) (engine.GateVerdict, error) {
		spy.gate++
		return engine.GateVerdict{
			ActionCheckID: "chk-refused-1", Governed: true, Allowed: false,
			Status: "needs_evidence", Reason: "required evidence does not qualify",
		}, nil
	}
	c := startDoctorActor(t, cfg)

	// resolve_finding first, so the execute step sees the resolved subject the
	// gate is asked about — exactly as the workflow threads it.
	_, resolveDelta := callAction(t, c, "doctor.resolve_finding",
		map[string]any{"finding_id": "f-1", "step_index": 0,
			"finding_binding_mode": engine.FindingBindingOperatorCurrent},
		map[string]any{})

	resp, delta := callAction(t, c, "doctor.execute_remediation",
		map[string]any{"finding_id": "f-1", "step_index": 0, "dry_run": false},
		resolveDelta)

	if resp.GetOk() {
		t.Fatal("a governed refusal must not report Ok")
	}
	dr, ok := delta["dispatch_result"].(map[string]any)
	if !ok {
		t.Fatalf("the refusal receipt MUST cross the wire on a failed callback; OutputJson=%q. "+
			"Without it the classifier sees an executor failure and charges the circuit breaker "+
			"for a decision that was working correctly", resp.GetOutputJson())
	}
	if dr["disposition"] != engine.DispositionRefused {
		t.Errorf("disposition = %v, want %q", dr["disposition"], engine.DispositionRefused)
	}
	if dr["action_check_id"] != "chk-refused-1" {
		t.Errorf("action_check_id = %v, want chk-refused-1", dr["action_check_id"])
	}
	if dr["status"] != "needs_evidence" {
		t.Errorf("governance status = %v, want needs_evidence", dr["status"])
	}
	if resp.GetMessage() == "" {
		t.Error("the human-readable failure must remain in Message, separate from the receipt")
	}
	if spy.exec != 0 || spy.verify != 0 || spy.observe != 0 {
		t.Errorf("refusal must touch nothing downstream: exec=%d verify=%d observe=%d",
			spy.exec, spy.verify, spy.observe)
	}
	if spy.gate != 1 {
		t.Errorf("gate consulted %d time(s), want exactly 1", spy.gate)
	}
}

// TestActorTransport_GovernanceUnavailableSurvivesFailedCallback proves the
// outage receipt crosses too, and stays distinct from a refusal.
func TestActorTransport_GovernanceUnavailableSurvivesFailedCallback(t *testing.T) {
	cfg, spy := transportConfig(t)
	cfg.GateAction = func(context.Context, engine.GateRequest) (engine.GateVerdict, error) {
		spy.gate++
		return engine.GateVerdict{}, fmt.Errorf("behavioral memory unreachable")
	}
	c := startDoctorActor(t, cfg)

	_, resolveDelta := callAction(t, c, "doctor.resolve_finding",
		map[string]any{"finding_id": "f-1", "step_index": 0,
			"finding_binding_mode": engine.FindingBindingOperatorCurrent},
		map[string]any{})
	resp, delta := callAction(t, c, "doctor.execute_remediation",
		map[string]any{"finding_id": "f-1", "step_index": 0, "dry_run": false},
		resolveDelta)

	if resp.GetOk() {
		t.Fatal("an unreachable governor must not report Ok")
	}
	dr, ok := delta["dispatch_result"].(map[string]any)
	if !ok {
		t.Fatalf("the outage receipt must cross the wire; OutputJson=%q", resp.GetOutputJson())
	}
	if dr["disposition"] != engine.DispositionExecutionFailed {
		t.Errorf("disposition = %v, want %q — an outage is not a decision",
			dr["disposition"], engine.DispositionExecutionFailed)
	}
	if unavailable, _ := dr["governance_unavailable"].(bool); !unavailable {
		t.Error("governance_unavailable must survive, so an outage stays distinguishable " +
			"from a stream of legitimate refusals")
	}
	if spy.exec != 0 || spy.observe != 0 {
		t.Errorf("nothing may run: exec=%d observe=%d", spy.exec, spy.observe)
	}
}

// TestActorTransport_MalformedOutputsFailClosed proves the actor refuses to run
// a handler against a fabricated view of the run.
func TestActorTransport_MalformedOutputsFailClosed(t *testing.T) {
	cfg, spy := transportConfig(t)
	c := startDoctorActor(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.ExecuteAction(ctx, &workflowpb.ExecuteActionRequest{
		RunId: "run-bad", Actor: "cluster-doctor", Action: "doctor.resolve_finding",
		WithJson: `{"finding_id":"f-1"}`, OutputsJson: `{ this is not json`,
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if resp.GetOk() {
		t.Error("malformed outputs must fail closed, not proceed with an empty map")
	}
	if spy.resolve != 0 {
		t.Error("the handler must not run against a fabricated view of the run")
	}
}

// TestAutonomousEndpoint_NeverAdvertisesLoopback proves finding 3.
func TestAutonomousEndpoint_NeverAdvertisesLoopback(t *testing.T) {
	for _, addr := range []string{
		"localhost:10300", "127.0.0.1:10300", "[::1]:10300", "0.0.0.0:10300", "",
	} {
		t.Run("rejects "+addr, func(t *testing.T) {
			srv := &ClusterDoctorServer{
				clusterID:             "c1",
				cfg:                   &clusterdoctorConfig{Port: 10300},
				actorEndpointResolver: func() string { return addr },
			}
			if _, err := srv.resolveActorEndpoint(); err == nil {
				t.Errorf("endpoint %q must be refused — a workflow service on another node "+
					"resolves it as its own host and can never call this doctor back", addr)
			}
		})
	}

	t.Run("accepts a registered routable address", func(t *testing.T) {
		srv := &ClusterDoctorServer{
			clusterID:             "c1",
			cfg:                   &clusterdoctorConfig{Port: 10300},
			actorEndpointResolver: func() string { return "10.0.0.63:10300" },
		}
		got, err := srv.resolveActorEndpoint()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "10.0.0.63:10300" {
			t.Errorf("endpoint = %q, want the registered address verbatim", got)
		}
	})
}

// TestAutonomousEndpoint_FailsClosedBeforeStartingWorkflow proves an
// unreachable callback stops the run BEFORE it burns a run id, a governance
// decision and a lease on a run that could only die at its first actor step.
func TestAutonomousEndpoint_FailsClosedBeforeStartingWorkflow(t *testing.T) {
	fake := &fakeWorkflowClient{}
	srv := &ClusterDoctorServer{
		workflowClient:        fake,
		clusterID:             "c1",
		cfg:                   &clusterdoctorConfig{Port: 10300},
		actorEndpointResolver: func() string { return "" },
	}
	srv.isAuthoritative.Store(true)

	f := autonomousFinding()
	res, err := srv.runAutonomousRemediation(context.Background(), f, 0, false)
	if err == nil {
		t.Fatal("a missing callback endpoint must fail closed")
	}
	if len(fake.requests) != 0 {
		t.Error("Workflow Service must not be called when the callback is unreachable")
	}
	if res.WorkflowRunID != "" {
		t.Error("no run was started, so no run id may be claimed")
	}
	if n := srv.runFindings.outstanding(); n != 0 {
		t.Errorf("%d binding(s) leaked on the fail-closed path", n)
	}
}

// TestAutonomousEndpoint_AdvertisesRoutableAddress proves the request actually
// carries the registered address.
func TestAutonomousEndpoint_AdvertisesRoutableAddress(t *testing.T) {
	fake := &fakeWorkflowClient{}
	srv := autonomousServer(fake)
	if _, err := srv.runAutonomousRemediation(context.Background(), autonomousFinding(), 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fake.requests[0].GetActorEndpoints()["cluster-doctor"]
	if got != "10.0.0.63:10300" {
		t.Errorf("advertised callback = %q, want the registered routable address", got)
	}
}

// TestBoundFindingReachesTheExecutor_CallbackLevel proves the identity hole
// Codex found: the run-scoped binding guarded resolve_finding, but the execute
// callback went through the PUBLIC ExecuteRemediation RPC, which re-resolves
// from the mutable lastFindings cache.
//
// The binding therefore protected the half that does not touch the cluster.
// Between resolve and execute the cache can be republished, so the mutation
// could act on a different subject than the healer bound — with the same
// finding id and a different node and unit.
//
// SCOPE: this is a CALLBACK-LEVEL test. It invokes cfg.ExecuteRemediation
// directly and does NOT cross the actor RPC, despite living in this file. The
// remote proof — the full remote DAG over bufconn — is still missing; see the
// outstanding transport-proof work on #236. Naming it accurately matters more
// than the convenience of the file it sits in: a comment claiming a boundary
// the test never crosses is how the last two rounds of defects survived
// review.
func TestBoundFindingReachesTheExecutor_CallbackLevel(t *testing.T) {
	const runID = "run-bound-exec"

	findingA := autonomousFinding() // node-1 / globular-log.service

	// Same finding id, different subject and action — what a later evaluation
	// cycle could publish into the cache.
	findingB := autonomousFinding()
	findingB.EntityRef = "node-9/globular-attacker.service"
	findingB.Remediation[0].Action.Params = map[string]string{
		"node_id": "node-9", "unit": "globular-attacker.service",
	}

	var executed []rules.Finding
	srv := &ClusterDoctorServer{
		clusterID:             "c1",
		cfg:                   &clusterdoctorConfig{Port: 10300},
		actorEndpointResolver: func() string { return "10.0.0.63:10300" },
		lastFindings:          []rules.Finding{findingB}, // the impostor is what the cache holds
	}
	srv.isAuthoritative.Store(true)
	srv.executeForFindingHook = func(f rules.Finding) (*cluster_doctorpb.ExecuteRemediationResponse, error) {
		executed = append(executed, f)
		return &cluster_doctorpb.ExecuteRemediationResponse{
			AuditId: "rem-bound", Executed: true, Status: "executed",
		}, nil
	}

	if err := srv.runFindings.bind(runID, findingA, 0); err != nil {
		t.Fatalf("bind: %v", err)
	}
	defer srv.runFindings.release(runID)

	cfg := srv.buildDoctorRemediationConfig()
	res, err := cfg.ExecuteRemediation(context.Background(), runID, findingA.FindingID, 0, "", false,
		engine.FindingBindingAutonomousRequired)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.Executed {
		t.Fatal("the bound finding must execute")
	}
	if len(executed) != 1 {
		t.Fatalf("executor called %d time(s), want exactly 1", len(executed))
	}
	got := executed[0]
	if got.EntityRef != findingA.EntityRef {
		t.Errorf("executed subject = %q, want %q — the cache substituted a different finding "+
			"under the same id, and the mutation followed it", got.EntityRef, findingA.EntityRef)
	}
	if p := got.Remediation[0].GetAction().GetParams(); p["node_id"] != "node-1" ||
		p["unit"] != "globular-log.service" {
		t.Errorf("executed action params = %v, want the BOUND finding's action — a reconstructed "+
			"view could differ in exactly the fields that decide what gets mutated", p)
	}
}

// TestExecutionFailsClosedWithoutBinding_CallbackLevel proves a missing or
// mismatched binding stops the mutation instead of falling back to the cache.
//
// SCOPE: callback-level, like the test above — it does not cross the actor RPC.
func TestExecutionFailsClosedWithoutBinding_CallbackLevel(t *testing.T) {
	f := autonomousFinding()
	calls := 0
	srv := &ClusterDoctorServer{
		clusterID:             "c1",
		cfg:                   &clusterdoctorConfig{Port: 10300},
		actorEndpointResolver: func() string { return "10.0.0.63:10300" },
		lastFindings:          []rules.Finding{f}, // cache COULD satisfy a fallback
	}
	srv.isAuthoritative.Store(true)
	srv.executeForFindingHook = func(rules.Finding) (*cluster_doctorpb.ExecuteRemediationResponse, error) {
		calls++
		return &cluster_doctorpb.ExecuteRemediationResponse{Executed: true}, nil
	}
	cfg := srv.buildDoctorRemediationConfig()

	t.Run("no binding", func(t *testing.T) {
		_, err := cfg.ExecuteRemediation(context.Background(), "run-none", f.FindingID, 0, "", false,
			engine.FindingBindingAutonomousRequired)
		if err == nil {
			t.Error("a missing binding must fail closed before mutation")
		}
	})

	t.Run("wrong finding", func(t *testing.T) {
		const runID = "run-wrong"
		if err := srv.runFindings.bind(runID, f, 0); err != nil {
			t.Fatalf("bind: %v", err)
		}
		defer srv.runFindings.release(runID)
		if _, err := cfg.ExecuteRemediation(context.Background(), runID, "f-other", 0, "", false,
			engine.FindingBindingAutonomousRequired); err == nil {
			t.Error("a mismatched finding must fail closed before mutation")
		}
	})

	t.Run("operator mode on a bound run", func(t *testing.T) {
		const runID = "run-downgrade"
		if err := srv.runFindings.bind(runID, f, 0); err != nil {
			t.Fatalf("bind: %v", err)
		}
		defer srv.runFindings.release(runID)
		if _, err := cfg.ExecuteRemediation(context.Background(), runID, f.FindingID, 0, "", false,
			engine.FindingBindingOperatorCurrent); err == nil {
			t.Error("a bound run must not be executed through the current-finding path")
		}
	})

	if calls != 0 {
		t.Errorf("executor called %d time(s) across fail-closed cases, want 0", calls)
	}
}

// TestNoLoopbackInCentralizedRemediationPaths is a static guard: no non-test Go
// file in this package may construct a loopback or unspecified callback
// address.
//
// Both centralized remediation entry points (autonomous dispatch and the public
// StartRemediationWorkflow) advertise an endpoint that a Workflow Service on
// ANOTHER node must dial. A loopback literal resolves to that node's own host,
// so the callback silently never arrives and the run dies at its first actor
// step. Behavioural tests cover the two current call sites; this catches a
// third one being added later.
func TestNoLoopbackInCentralizedRemediationPaths(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	// Only the ADVERTISEMENT shape — a literal loopback address being built for
	// someone else to dial. Deliberately narrow:
	//
	//   0.0.0.0 is correct for BIND/listen and appears legitimately in main.go;
	//   "::1" and "127.0.0.1" appear legitimately inside rejectUnroutableCallback's
	//   own deny-list and in TLS server-name handling.
	//
	// A guard that flagged those would be noise, and noisy guards get deleted.
	// This catches the exact construction that was removed twice:
	// fmt.Sprintf("localhost:%d", port).
	banned := []string{`"localhost:`, "localhost:%d"}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		scanned++
		for _, lit := range banned {
			if strings.Contains(string(b), lit) {
				t.Errorf("%s constructs a loopback address (%s). A workflow service on another "+
					"node resolves it as its own host and can never call this doctor back — use "+
					"resolveActorEndpoint().", name, lit)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no source files — the guard would pass vacuously")
	}
}
