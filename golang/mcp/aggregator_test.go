package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newAggTestServer creates an httptest server that responds to POST /mcp with
// standard JSON-RPC MCP responses. handlers maps tool name to return value.
func newAggTestServer(t *testing.T, handlers map[string]interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req aggRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if req.Method == "ping" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]interface{}{},
			})
			return
		}

		if req.Method == "tools/call" {
			var params aggToolCallParams
			raw, _ := json.Marshal(req.Params)
			json.Unmarshal(raw, &params)

			if result, ok := handlers[params.Name]; ok {
				text, _ := json.Marshal(result)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]interface{}{
						"content": []map[string]interface{}{
							{"type": "text", "text": string(text)},
						},
						"isError": false,
					},
				})
				return
			}
			// Unknown tool
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error": map[string]interface{}{
					"code":    -32602,
					"message": fmt.Sprintf("unknown tool: %s", params.Name),
				},
			})
		}
	}))
	return srv
}

// ── Test 1: All aggregator tools are registered ───────────────────────────────

func TestAggregatorToolsRegistered(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Aggregator = true
	// Disable groups that need live services to avoid side effects.
	cfg.ToolGroups.Cluster = false
	cfg.ToolGroups.Doctor = false
	cfg.ToolGroups.NodeAgent = false
	cfg.ToolGroups.Repository = false
	cfg.ToolGroups.Backup = false
	cfg.ToolGroups.RBAC = false
	cfg.ToolGroups.Resource = false
	cfg.ToolGroups.File = false
	cfg.ToolGroups.Composed = false
	cfg.ToolGroups.CLI = false
	cfg.ToolGroups.Governor = false
	cfg.ToolGroups.Memory = false
	cfg.ToolGroups.Skills = false
	cfg.ToolGroups.Workflow = false
	cfg.ToolGroups.Etcd = false
	cfg.ToolGroups.Title = false
	cfg.ToolGroups.Frontend = false
	cfg.ToolGroups.Proto = false
	cfg.ToolGroups.HTTPDiag = false
	cfg.ToolGroups.Monitoring = false
	cfg.ToolGroups.Browser = false
	cfg.ToolGroups.AIExecutor = false
	cfg.ToolGroups.Awareness = false

	s := newServer(cfg)
	registerAllTools(s)

	expected := []string{
		"mcp.cluster_nodes",
		"mcp.remote_ping",
		"mcp.remote_call",
		"mcp.remote_snapshot",
		"mcp.compare_nodes",
		"mcp.cluster_snapshot",
		"mcp.day1_classify_node",
	}
	for _, name := range expected {
		if !s.hasTool(name) {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

// ── Test 2: Existing local tools still register alongside aggregator ──────────

func TestAggregatorDoesNotBreakExistingTools(t *testing.T) {
	cfg := defaultConfig()
	// Enable both awareness (a local group) and aggregator.
	cfg.ToolGroups.Aggregator = true
	cfg.ToolGroups.Awareness = true
	// Disable live-network groups.
	cfg.ToolGroups.Cluster = false
	cfg.ToolGroups.Doctor = false
	cfg.ToolGroups.NodeAgent = false
	cfg.ToolGroups.Repository = false
	cfg.ToolGroups.Backup = false
	cfg.ToolGroups.RBAC = false
	cfg.ToolGroups.Resource = false
	cfg.ToolGroups.File = false
	cfg.ToolGroups.Composed = false
	cfg.ToolGroups.CLI = false
	cfg.ToolGroups.Governor = false
	cfg.ToolGroups.Memory = false
	cfg.ToolGroups.Skills = false
	cfg.ToolGroups.Workflow = false
	cfg.ToolGroups.Etcd = false
	cfg.ToolGroups.Title = false
	cfg.ToolGroups.Frontend = false
	cfg.ToolGroups.Proto = false
	cfg.ToolGroups.HTTPDiag = false
	cfg.ToolGroups.Monitoring = false
	cfg.ToolGroups.Browser = false
	cfg.ToolGroups.AIExecutor = false

	s := newServer(cfg)
	registerAllTools(s)

	// Aggregator tools present.
	if !s.hasTool("mcp.cluster_nodes") {
		t.Error("mcp.cluster_nodes not registered")
	}
	// A known awareness tool still present (was registered before aggregator).
	if !s.hasTool("awareness.bundle_status") {
		t.Error("awareness.bundle_status missing — aggregator must not remove existing tools")
	}
}

// ── Test 3: Remote ping success against fake server ───────────────────────────

func TestRemotePingSuccess(t *testing.T) {
	srv := newAggTestServer(t, nil) // ping handler is built into newAggTestServer
	defer srv.Close()

	// Use the plain HTTP test URL; replace https with http for the test client.
	mcpURL := srv.URL

	// For testing we bypass TLS (test server is plain HTTP).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call doRemoteCall directly (bypasses registry + TLS layer).
	req := aggRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping"}
	resp, _, err := sendHTTPMCPRequest(ctx, mcpURL+"/mcp", mustMarshal(req), aggPingTimeout, true)
	if err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("ping returned error: %v", resp.Error.Message)
	}
}

// ── Test 4: Policy — allowlist blocks unsafe tools ────────────────────────────

func TestAllowlistBlocksForbiddenTools(t *testing.T) {
	forbidden := []string{
		"etcd_put",
		"etcd_delete",
		"nodeagent_control_service",
		"nodeagent_installed_set",
		"repository_publish",
		"backup_restore",
		"file_write",
		"file_delete",
		"workflow_execute",
	}
	for _, tool := range forbidden {
		if IsRemoteToolAllowed(tool) {
			t.Errorf("tool %q should be blocked by allowlist but is allowed", tool)
		}
	}
}

// ── Test 5: Policy — allowlist permits safe read-only tools ──────────────────

func TestAllowlistPermitsSafeTools(t *testing.T) {
	safe := []string{
		"awareness.bundle_status",
		"awareness.runtime_errors",
		"awareness.day1_classify_node",
		"nodeagent_get_inventory",
		"nodeagent_list_installed_packages",
		"nodeagent_get_certificate_status",
	}
	for _, tool := range safe {
		if !IsRemoteToolAllowed(tool) {
			t.Errorf("tool %q should be allowed but is blocked", tool)
		}
	}
}

// ── Test 6: mcp.remote_call rejects forbidden tool before network call ────────

func TestRemoteCallDeniedTool(t *testing.T) {
	// The test server should never be reached for a forbidden tool.
	contacted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contacted = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := defaultConfig()
	cfg.ToolGroups.Aggregator = true
	// Disable all groups to avoid network deps.
	cfg.ToolGroups.Cluster = false
	cfg.ToolGroups.Doctor = false
	cfg.ToolGroups.NodeAgent = false
	cfg.ToolGroups.Repository = false
	cfg.ToolGroups.Backup = false
	cfg.ToolGroups.RBAC = false
	cfg.ToolGroups.Resource = false
	cfg.ToolGroups.File = false
	cfg.ToolGroups.Composed = false
	cfg.ToolGroups.CLI = false
	cfg.ToolGroups.Governor = false
	cfg.ToolGroups.Memory = false
	cfg.ToolGroups.Skills = false
	cfg.ToolGroups.Workflow = false
	cfg.ToolGroups.Etcd = false
	cfg.ToolGroups.Title = false
	cfg.ToolGroups.Frontend = false
	cfg.ToolGroups.Proto = false
	cfg.ToolGroups.HTTPDiag = false
	cfg.ToolGroups.Monitoring = false
	cfg.ToolGroups.Browser = false
	cfg.ToolGroups.AIExecutor = false
	cfg.ToolGroups.Awareness = false

	s := newServer(cfg)
	registerAggregatorTools(s)

	// Override registry for this test using a fake node entry.
	// We inject via direct policy check, not via the registry (no etcd/controller).
	result, err := s.callTool(context.Background(), "mcp.remote_call", map[string]interface{}{
		"node_id": "node-b",
		"tool":    "etcd_delete",
		"args":    map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("tool returned unexpected error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["error_kind"] != ErrMCPToolNotAllowed {
		t.Errorf("expected error_kind=%s, got %v", ErrMCPToolNotAllowed, m["error_kind"])
	}
	if contacted {
		t.Error("remote server was contacted despite the tool being forbidden — allowlist not enforced before network call")
	}
}

// ── Test 7: callRemoteTool returns structured result from fake MCP server ─────

func TestCallRemoteToolSuccess(t *testing.T) {
	expected := map[string]interface{}{
		"bundle_version": "v1.2.30",
		"status":         "LOADED",
	}
	srv := newAggTestServer(t, map[string]interface{}{
		"awareness.bundle_status": expected,
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, trust, err := callRemoteTool(ctx, srv.URL, "awareness.bundle_status", nil)
	if err != nil {
		t.Fatalf("callRemoteTool failed: %v", err)
	}
	_ = trust

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T: %v", result, result)
	}
	if m["bundle_version"] != "v1.2.30" {
		t.Errorf("expected bundle_version=v1.2.30, got %v", m["bundle_version"])
	}
}

// ── Test 8: pingRemoteMCP fails gracefully when server is unreachable ─────────

func TestPingRemoteMCPUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reachable, trust, _, err := pingRemoteMCP(ctx, "https://127.0.0.1:19999")
	if reachable {
		t.Error("expected unreachable, got reachable")
	}
	if trust != MCPTrustNone {
		t.Errorf("expected trust=NONE, got %s", trust)
	}
	if err == nil {
		t.Error("expected error for unreachable host, got nil")
	}
}

// ── Test 9: compareNodes detects version mismatch ─────────────────────────────

func TestDeepEqualMismatchDetected(t *testing.T) {
	a := map[string]interface{}{"version": "v1.2.30", "build_id": "aaa"}
	b := map[string]interface{}{"version": "v1.2.29", "build_id": "bbb"}
	if deepEqual(a, b) {
		t.Error("deepEqual should return false for different versions")
	}
}

func TestDeepEqualMatchPasses(t *testing.T) {
	a := map[string]interface{}{"version": "v1.2.30"}
	b := map[string]interface{}{"version": "v1.2.30"}
	if !deepEqual(a, b) {
		t.Error("deepEqual should return true for identical values")
	}
}

// ── Test 10: policy ClassifyRemoteToolSafety ──────────────────────────────────

func TestClassifyRemoteToolSafety(t *testing.T) {
	cases := []struct {
		tool     string
		expected string
	}{
		{"awareness.bundle_status", "READ_ONLY"},
		{"etcd_delete", "FORBIDDEN"},
		{"some_unknown_tool", "NOT_ALLOWLISTED"},
	}
	for _, c := range cases {
		got := ClassifyRemoteToolSafety(c.tool)
		if got != c.expected {
			t.Errorf("ClassifyRemoteToolSafety(%q) = %q, want %q", c.tool, got, c.expected)
		}
	}
}

// ── Test 11: extractIP helper ─────────────────────────────────────────────────

func TestExtractIP(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"10.0.0.8:11000", "10.0.0.8"},
		{"10.0.0.63:11000", "10.0.0.63"},
		{"", ""},
		{"10.0.0.1", "10.0.0.1"},
		{"[::1]:11000", "::1"},
	}
	for _, c := range cases {
		got := extractIP(c.in)
		if got != c.out {
			t.Errorf("extractIP(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

// ── Test 12: etcd override port preference ────────────────────────────────────

// When /globular/mcp/nodes/<node-id> publishes a non-canonical MCP port (e.g.
// 10060), the aggregator must prefer that override over the derived 10260
// default. mergeNodeOverrides is the testable seam in listMCPNodes.
func TestEtcdOverridePortPreferredOverDerived(t *testing.T) {
	nodes := []*cluster_controllerpb.NodeRecord{
		{
			NodeId:        "node-a",
			AgentEndpoint: "10.0.0.8:11000",
			Identity:      &cluster_controllerpb.NodeIdentity{Hostname: "node-a"},
			Status:        "ACTIVE",
		},
		{
			NodeId:        "node-b",
			AgentEndpoint: "10.0.0.20:11000",
			Identity:      &cluster_controllerpb.NodeIdentity{Hostname: "node-b"},
			Status:        "ACTIVE",
		},
	}

	// node-a publishes a custom port (10060). node-b has no override → derived 10260.
	overrides := map[string]MCPNodeEntry{
		"node-a": {
			NodeID:   "node-a",
			Hostname: "node-a",
			IP:       "10.0.0.8",
			MCPURL:   "https://10.0.0.8:10060",
			MCPPort:  10060,
			Status:   "RUNNING",
		},
	}

	merged := mergeNodeOverrides(nodes, overrides)
	if len(merged) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(merged))
	}

	var a, b *MCPNodeEntry
	for i := range merged {
		switch merged[i].NodeID {
		case "node-a":
			a = &merged[i]
		case "node-b":
			b = &merged[i]
		}
	}
	if a == nil || b == nil {
		t.Fatalf("expected node-a and node-b in merged result")
	}

	if a.MCPPort != 10060 {
		t.Errorf("node-a MCP port = %d, want 10060 (etcd override)", a.MCPPort)
	}
	if a.MCPURL != "https://10.0.0.8:10060" {
		t.Errorf("node-a MCP URL = %q, want https://10.0.0.8:10060 (etcd override)", a.MCPURL)
	}
	if b.MCPPort != aggregatorMCPPort {
		t.Errorf("node-b MCP port = %d, want %d (canonical default)", b.MCPPort, aggregatorMCPPort)
	}
	if !strings.Contains(b.MCPURL, ":10260") {
		t.Errorf("node-b MCP URL = %q, want canonical :10260", b.MCPURL)
	}
}

// ── Test 13: untrusted TLS blocks DAY1_COMPLETE ───────────────────────────────
//
// The aggregator MUST NOT inherit a PASS verdict from a node whose TLS
// identity cannot be verified against the cluster CA. These tests pin that
// invariant for both the pre-call gate (applyTrustGate) and the post-call
// downgrade case (applyRemoteVerdict).

// applyTrustGate: trust=UNVERIFIED → BLOCK, no remote call.
func TestApplyTrustGateUnverifiedBlocksDay1Complete(t *testing.T) {
	out := map[string]interface{}{"node_id": "node-x"}
	final := applyTrustGate(out, "node-x", true, MCPTrustUnverified)

	if !final {
		t.Fatal("applyTrustGate must return final=true when trust is UNVERIFIED — caller must not proceed to remote call")
	}
	if out["aggregator_verdict"] != "BLOCK" {
		t.Errorf("verdict = %v, want BLOCK", out["aggregator_verdict"])
	}
	if out["aggregator_classification"] != "MCP_REACHABLE_BUT_UNTRUSTED" {
		t.Errorf("classification = %v, want MCP_REACHABLE_BUT_UNTRUSTED", out["aggregator_classification"])
	}
	if out["error_kind"] != ErrMCPTLSUntrusted {
		t.Errorf("error_kind = %v, want %s", out["error_kind"], ErrMCPTLSUntrusted)
	}
	// Forbidden actions must explicitly forbid marking DAY1_COMPLETE.
	forbidden, _ := out["forbidden_actions"].([]string)
	found := false
	for _, a := range forbidden {
		if a == "mark node DAY1_COMPLETE" {
			found = true
		}
	}
	if !found {
		t.Errorf("forbidden_actions must include \"mark node DAY1_COMPLETE\"; got %v", forbidden)
	}
}

// applyTrustGate: not reachable → BLOCK / MCP_UNREACHABLE.
func TestApplyTrustGateUnreachableBlocks(t *testing.T) {
	out := map[string]interface{}{"node_id": "node-x"}
	final := applyTrustGate(out, "node-x", false, MCPTrustNone)

	if !final {
		t.Fatal("applyTrustGate must return final=true when not reachable")
	}
	if out["aggregator_verdict"] != "BLOCK" {
		t.Errorf("verdict = %v, want BLOCK", out["aggregator_verdict"])
	}
	if out["aggregator_classification"] != "MCP_UNREACHABLE" {
		t.Errorf("classification = %v, want MCP_UNREACHABLE", out["aggregator_classification"])
	}
}

// applyTrustGate: trust=VERIFIED → not final (caller proceeds to remote call).
func TestApplyTrustGateVerifiedProceeds(t *testing.T) {
	out := map[string]interface{}{"node_id": "node-x"}
	final := applyTrustGate(out, "node-x", true, MCPTrustVerified)

	if final {
		t.Fatal("applyTrustGate must return final=false when trust is VERIFIED — caller proceeds to remote call")
	}
	if _, ok := out["aggregator_verdict"]; ok {
		t.Error("verdict must NOT be set when trust is VERIFIED — caller decides based on remote response")
	}
}

// applyRemoteVerdict: even when remote claims PASS, an UNVERIFIED trust
// downgrade during the call must produce BLOCK / MCP_REACHABLE_BUT_UNTRUSTED.
// This pins the invariant that a remote PASS is never inherited unless
// the transport stayed verified end-to-end.
func TestApplyRemoteVerdictUnverifiedTrustOverridesPass(t *testing.T) {
	out := map[string]interface{}{"node_id": "node-x"}
	remoteClaimsPass := map[string]interface{}{
		"verdict":        "PASS",
		"classification": "DAY1_COMPLETE",
	}

	applyRemoteVerdict(out, remoteClaimsPass, MCPTrustUnverified, nil)

	if out["aggregator_verdict"] != "BLOCK" {
		t.Errorf("aggregator must override remote PASS with BLOCK when trust is UNVERIFIED; got %v", out["aggregator_verdict"])
	}
	if out["aggregator_classification"] != "MCP_REACHABLE_BUT_UNTRUSTED" {
		t.Errorf("classification = %v, want MCP_REACHABLE_BUT_UNTRUSTED", out["aggregator_classification"])
	}
	if out["error_kind"] != ErrMCPTLSUntrusted {
		t.Errorf("error_kind = %v, want %s", out["error_kind"], ErrMCPTLSUntrusted)
	}
	// remote_verdict should still be attached for transparency.
	if out["remote_verdict"] == nil {
		t.Error("remote_verdict must still be attached so the operator can see what the node claimed")
	}
}

// applyRemoteVerdict: with VERIFIED trust and a remote PASS, aggregator passes.
func TestApplyRemoteVerdictVerifiedPassFlowsThrough(t *testing.T) {
	out := map[string]interface{}{"node_id": "node-x"}
	remote := map[string]interface{}{
		"verdict":        "PASS",
		"classification": "DAY1_COMPLETE",
	}
	applyRemoteVerdict(out, remote, MCPTrustVerified, nil)

	if out["aggregator_verdict"] != "PASS" {
		t.Errorf("verdict = %v, want PASS", out["aggregator_verdict"])
	}
	if out["aggregator_classification"] != "DAY1_COMPLETE" {
		t.Errorf("classification = %v, want DAY1_COMPLETE", out["aggregator_classification"])
	}
}

// applyRemoteVerdict: BLOCK from remote with verified trust → BLOCK + inherit blocker.
func TestApplyRemoteVerdictVerifiedBlockInheritsBlocker(t *testing.T) {
	out := map[string]interface{}{"node_id": "node-x"}
	remote := map[string]interface{}{
		"verdict":         "BLOCK",
		"classification":  "SCYLLA_NOT_READY",
		"primary_blocker": "scylla-server.service is failed",
	}
	applyRemoteVerdict(out, remote, MCPTrustVerified, nil)

	if out["aggregator_verdict"] != "BLOCK" {
		t.Errorf("verdict = %v, want BLOCK", out["aggregator_verdict"])
	}
	if out["aggregator_classification"] != "SCYLLA_NOT_READY" {
		t.Errorf("classification = %v, want SCYLLA_NOT_READY", out["aggregator_classification"])
	}
	if out["primary_blocker"] != "scylla-server.service is failed" {
		t.Errorf("primary_blocker = %v, want \"scylla-server.service is failed\"", out["primary_blocker"])
	}
}

// extractRemoteVerdictFields handles both decoded map and raw JSON shapes.
func TestExtractRemoteVerdictFields(t *testing.T) {
	cases := []struct {
		name           string
		input          interface{}
		wantVerdict    string
		wantClass      string
		wantBlocker    string
	}{
		{
			name: "map with all fields",
			input: map[string]interface{}{
				"verdict":         "BLOCK",
				"classification":  "PKI_MISSING",
				"primary_blocker": "missing CA cert",
			},
			wantVerdict: "BLOCK",
			wantClass:   "PKI_MISSING",
			wantBlocker: "missing CA cert",
		},
		{
			name:        "json.RawMessage",
			input:       json.RawMessage(`{"verdict":"PASS","classification":"DAY1_COMPLETE"}`),
			wantVerdict: "PASS",
			wantClass:   "DAY1_COMPLETE",
		},
		{
			name:  "nil input",
			input: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, cl, b := extractRemoteVerdictFields(c.input)
			if v != c.wantVerdict {
				t.Errorf("verdict = %q, want %q", v, c.wantVerdict)
			}
			if cl != c.wantClass {
				t.Errorf("classification = %q, want %q", cl, c.wantClass)
			}
			if b != c.wantBlocker {
				t.Errorf("blocker = %q, want %q", b, c.wantBlocker)
			}
		})
	}
}

// ── Test 14: buildMCPURL ──────────────────────────────────────────────────────

func TestBuildMCPURL(t *testing.T) {
	url := buildMCPURL("10.0.0.8", 10260)
	if !strings.HasPrefix(url, "https://10.0.0.8:10260") {
		t.Errorf("unexpected MCP URL: %s", url)
	}
	if buildMCPURL("", 10260) != "" {
		t.Error("empty IP should produce empty URL")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshal: %v", err))
	}
	return b
}

// sendHTTPMCPRequest is exported to tests via the same package.
// The function signature uses plain HTTP for test servers (skipVerify=true).
// Production callers use doRemoteCall which tries verified TLS first.
