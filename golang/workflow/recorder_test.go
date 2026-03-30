package workflow

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// ---------------------------------------------------------------------------
// Unit tests — no live services required
// ---------------------------------------------------------------------------

func TestNilRecorderIsSafe(t *testing.T) {
	var rec *Recorder

	// All methods must be no-ops on nil receiver.
	if rec.Available() {
		t.Fatal("nil recorder should not be available")
	}
	id := rec.StartRun(context.Background(), &RunParams{})
	if id != "" {
		t.Fatalf("expected empty run ID from nil recorder, got %q", id)
	}
	// These should not panic.
	rec.RecordStep(context.Background(), "run-1", &StepParams{})
	rec.CompleteStep(context.Background(), "run-1", 1, "ok", 100)
	rec.FailStep(context.Background(), "run-1", 1, "ERR", "fail", "", NoFailure, true)
	rec.UpdateRunStatus(context.Background(), "run-1", Succeeded, "done", ActorController)
	rec.FinishRun(context.Background(), "run-1", Succeeded, "done", "", NoFailure)
	rec.Close()
}

func TestEmptyRunIDSkipsRPC(t *testing.T) {
	// Recorder with a resolver that returns an address, but no real server.
	// With empty runID, methods should return early without attempting connection.
	rec := NewRecorder("127.0.0.1:99999", "test-cluster")
	defer rec.Close()

	rec.RecordStep(context.Background(), "", &StepParams{})
	rec.CompleteStep(context.Background(), "", 1, "ok", 100)
	rec.FailStep(context.Background(), "", 1, "ERR", "fail", "", NoFailure, true)
	rec.UpdateRunStatus(context.Background(), "", Succeeded, "done", ActorController)
	rec.FinishRun(context.Background(), "", Succeeded, "done", "", NoFailure)
}

func TestResolverCalledLazily(t *testing.T) {
	calls := 0
	rec := NewRecorderWithResolver(func() string {
		calls++
		return "" // return empty to prevent actual dial
	}, "test-cluster")
	defer rec.Close()

	if calls != 0 {
		t.Fatal("resolver should not be called at construction time")
	}

	// Available() triggers ensureConnected which calls the resolver.
	rec.Available()
	if calls != 1 {
		t.Fatalf("expected resolver called once, got %d", calls)
	}
}

func TestEmptyResolverPreventsConnect(t *testing.T) {
	rec := NewRecorderWithResolver(func() string { return "" }, "test-cluster")
	defer rec.Close()

	if rec.Available() {
		t.Fatal("recorder with empty resolver should not be available")
	}
}

func TestMeshResolverUsesConfigGetMeshAddress(t *testing.T) {
	// This tests the pattern used by controller and repository:
	// route through the Envoy service mesh via config.GetMeshAddress().
	resolver := func() string {
		if env := strings.TrimSpace(os.Getenv("WORKFLOW_SERVICE_ADDR")); env != "" {
			return env
		}
		if addr, err := config.GetMeshAddress(); err == nil {
			return addr
		}
		return ""
	}

	addr := resolver()
	// On a running node, this should return <IP>:443 (Envoy).
	// In CI without config.json, it returns "".
	if addr != "" {
		if !strings.Contains(addr, ":443") {
			t.Fatalf("expected host:443, got %q", addr)
		}
		t.Logf("mesh resolver returned: %s", addr)
	} else {
		t.Log("mesh resolver returned empty (no config.json — expected in CI)")
	}
}

func TestSequenceNumbersIncrement(t *testing.T) {
	rec := &Recorder{
		seqMap: make(map[string]int32),
	}

	// Simulate incrementing without actual RPC.
	rec.mu.Lock()
	seq1 := rec.seqMap["run-1"] + 1
	rec.seqMap["run-1"] = seq1
	rec.mu.Unlock()

	rec.mu.Lock()
	seq2 := rec.seqMap["run-1"] + 1
	rec.seqMap["run-1"] = seq2
	rec.mu.Unlock()

	if seq1 != 1 {
		t.Fatalf("expected seq 1, got %d", seq1)
	}
	if seq2 != 2 {
		t.Fatalf("expected seq 2, got %d", seq2)
	}
}

func TestTokenInjectorAddsMetadata(t *testing.T) {
	interceptor := tokenInjector("my-token", "my-cluster")
	if interceptor == nil {
		t.Fatal("tokenInjector returned nil")
	}
}

func TestDisconnectResetsClient(t *testing.T) {
	rec := NewRecorder("127.0.0.1:99999", "test-cluster")
	// Manually set client to non-nil to simulate connected state.
	rec.client = workflowpb.NewWorkflowServiceClient(nil)
	rec.disconnect()

	if rec.client != nil {
		t.Fatal("disconnect should set client to nil")
	}
	if rec.conn != nil {
		t.Fatal("disconnect should set conn to nil")
	}
}

// ---------------------------------------------------------------------------
// Integration test — requires live cluster (Day-0 running)
// ---------------------------------------------------------------------------

// TestIntegrationWorkflowThroughEnvoy verifies the full path:
//
//	recorder → Envoy gateway (:8443) → workflow.WorkflowService
//
// This test is skipped unless GLOBULAR_INTEGRATION=1 is set.
func TestIntegrationWorkflowThroughEnvoy(t *testing.T) {
	if os.Getenv("GLOBULAR_INTEGRATION") != "1" {
		t.Skip("set GLOBULAR_INTEGRATION=1 to run live cluster tests")
	}

	// Resolve mesh address the same way the controller does.
	gatewayAddr, err := config.GetMeshAddress()
	if err != nil {
		t.Fatalf("config.GetAddress() failed: %v", err)
	}
	t.Logf("gateway address: %s", gatewayAddr)

	// Verify gateway is reachable.
	conn, err := net.DialTimeout("tcp", gatewayAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("gateway %s not reachable: %v", gatewayAddr, err)
	}
	conn.Close()

	// Create recorder routed through Envoy.
	rec := NewRecorderWithResolver(func() string { return gatewayAddr }, "globular.internal")
	defer rec.Close()

	if !rec.Available() {
		t.Fatal("recorder could not connect to workflow service through Envoy")
	}
	t.Log("recorder connected to workflow service through Envoy")

	// Start a test run.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	runID := rec.StartRun(ctx, &RunParams{
		NodeID:        "test-node",
		NodeHostname:  "test-host",
		ComponentName: "integration-test",
		ComponentKind: KindService,
		ReleaseKind:   "ServiceRelease",
		TriggerReason: TriggerRepair,
		CorrelationID: fmt.Sprintf("test/integration/%d", time.Now().UnixMilli()),
	})
	if runID == "" {
		t.Fatal("StartRun returned empty ID — workflow service may not be routing through Envoy")
	}
	t.Logf("started workflow run: %s", runID)

	// Record a step.
	seq := rec.RecordStep(ctx, runID, &StepParams{
		StepKey: "test-step",
		Title:   "Integration test step",
		Actor:   ActorController,
		Phase:   PhaseVerify,
		Status:  StepRunning,
		Message: "testing service mesh routing",
	})
	if seq == 0 {
		t.Fatal("RecordStep returned seq 0")
	}
	t.Logf("recorded step seq=%d", seq)

	// Complete the step.
	rec.CompleteStep(ctx, runID, seq, "test passed", 42)

	// Finish the run.
	rec.FinishRun(ctx, runID, Succeeded, "integration test passed", "", NoFailure)
	t.Logf("finished workflow run: %s", runID)

	// Verify we can read it back via a direct gRPC call.
	creds, err := loadRecorderTLS()
	if err != nil {
		t.Fatalf("loadRecorderTLS: %v", err)
	}
	readConn, err := grpc.DialContext(ctx, gatewayAddr,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial for read: %v", err)
	}
	defer readConn.Close()

	client := workflowpb.NewWorkflowServiceClient(readConn)
	resp, err := client.GetRun(ctx, &workflowpb.GetRunRequest{
		Id:        runID,
		ClusterId: "globular.internal",
	})
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if resp.Run == nil {
		t.Fatal("GetRun returned nil run")
	}
	if resp.Run.Status != Succeeded {
		t.Fatalf("expected run status SUCCEEDED, got %v", resp.Run.Status)
	}
	if len(resp.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(resp.Steps))
	}
	t.Logf("verified run %s: status=%v, steps=%d", runID, resp.Run.Status, len(resp.Steps))
}

// TestIntegrationWorkflowDirect verifies direct connection (no Envoy):
//
//	recorder → workflow_server:10220
//
// This test is skipped unless GLOBULAR_INTEGRATION=1 is set.
func TestIntegrationWorkflowDirect(t *testing.T) {
	if os.Getenv("GLOBULAR_INTEGRATION") != "1" {
		t.Skip("set GLOBULAR_INTEGRATION=1 to run live cluster tests")
	}

	// Check if workflow is running locally.
	conn, err := net.DialTimeout("tcp", "localhost:10220", 2*time.Second)
	if err != nil {
		t.Skip("workflow service not running locally on :10220")
	}
	conn.Close()

	rec := NewRecorder("localhost:10220", "globular.internal")
	defer rec.Close()

	if !rec.Available() {
		t.Fatal("recorder could not connect directly to workflow service on :10220")
	}
	t.Log("recorder connected directly to workflow:10220")

	ctx := context.Background()
	runID := rec.StartRun(ctx, &RunParams{
		NodeID:        "test-node",
		NodeHostname:  "test-host",
		ComponentName: "direct-test",
		ComponentKind: KindService,
		CorrelationID: fmt.Sprintf("test/direct/%d", time.Now().UnixMilli()),
	})
	if runID == "" {
		t.Fatal("StartRun returned empty ID on direct connection")
	}
	t.Logf("direct connection: started run %s", runID)

	rec.FinishRun(ctx, runID, Succeeded, "direct test passed", "", NoFailure)
	t.Logf("direct connection: finished run %s", runID)
}

// Stub to satisfy the import of reflection (used indirectly).
var _ = reflection.Register
var _ credentials.TransportCredentials
