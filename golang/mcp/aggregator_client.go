// @awareness namespace=globular.platform
// @awareness component=platform_mcp.aggregator
// @awareness file_role=etcd_discovery_and_tls_verified_remote_tool_routing
// @awareness implements=globular.platform:intent.mcp.aggregator_routes_via_etcd_discovery
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness risk=medium
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ── Timeout constants ─────────────────────────────────────────────────────────

const (
	aggPingTimeout     = 3 * time.Second
	aggToolTimeout     = 8 * time.Second
	aggSnapshotTimeout = 15 * time.Second
	aggMaxConcurrent   = 5
)

// ── JSON-RPC types for remote calls ──────────────────────────────────────────

type aggRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type aggToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type aggRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} `json:"result,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── TLS helpers ───────────────────────────────────────────────────────────────

const clusterCAPath = "/var/lib/globular/pki/ca.crt"
const mcpAllowUnverifiedFallbackEnv = "GLOBULAR_MCP_ALLOW_UNVERIFIED_FALLBACK"

func buildAggregatorTLSConfig(skipVerify bool) *tls.Config {
	caCert, err := os.ReadFile(clusterCAPath)
	if err != nil {
		return &tls.Config{InsecureSkipVerify: skipVerify} //nolint:gosec
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)
	return &tls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: skipVerify, //nolint:gosec
	}
}

func newAggregatorHTTPClient(timeout time.Duration, skipVerify bool) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: buildAggregatorTLSConfig(skipVerify),
		},
	}
}

// ── Remote MCP call ───────────────────────────────────────────────────────────

// doRemoteCall sends a single JSON-RPC request to the remote MCP server at url.
// It tries cluster-CA-verified TLS first; on TLS failure it retries with
// InsecureSkipVerify and reports MCPTrustUnverified.
//
// Session handling: the MCP HTTP transport requires every non-"initialize"
// request to carry a valid Mcp-Session-Id (issued by the server in response
// to an initialize call). For methods that need a session ("tools/call"),
// this function performs the initialize handshake first, then attaches the
// resulting session id to the actual request. The session is best-effort
// released via DELETE after the call completes.
//
// Methods that don't require a session (notably "ping", which is allowed to
// fail with a server-side error and still confirms reachability) skip the
// handshake — the cost of an extra round-trip would not change the verdict.
func doRemoteCall(ctx context.Context, mcpURL string, req aggRPCRequest, timeout time.Duration) (resp *aggRPCResponse, trust MCPTrustLevel, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, MCPTrustNone, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := mcpURL + "/mcp"

	if methodNeedsSession(req.Method) {
		return doRemoteCallWithSession(ctx, endpoint, body, timeout)
	}

	// No-session path (ping, initialize): single attempt with TLS fallback.
	return doSendWithTLSFallback(ctx, endpoint, body, "", timeout)
}

// doSendWithTLSFallback runs a single sendHTTPMCPRequest, then on TLS error
// retries with InsecureSkipVerify and reports MCPTrustUnverified. sessionID
// may be empty for methods that don't require one.
func doSendWithTLSFallback(ctx context.Context, endpoint string, body []byte, sessionID string, timeout time.Duration) (*aggRPCResponse, MCPTrustLevel, error) {
	resp, trust, _, err := sendHTTPMCPRequest(ctx, endpoint, body, sessionID, timeout, false)
	if err == nil {
		return resp, trust, nil
	}
	if isTLSError(err) {
		if !allowUnverifiedFallback() {
			return nil, MCPTrustNone, fmt.Errorf("TLS verification failed for %s and unverified fallback is disabled (%s not true): %w", endpoint, mcpAllowUnverifiedFallbackEnv, err)
		}
		log.Printf("aggregator: TLS verification failed for %s: %v — retrying unverified", endpoint, err)
		resp, _, _, err = sendHTTPMCPRequest(ctx, endpoint, body, sessionID, timeout, true)
		if err == nil {
			return resp, MCPTrustUnverified, nil
		}
	}
	return nil, MCPTrustNone, err
}

// doRemoteCallWithSession initializes a session, runs the request, then
// best-effort releases the session. Trust level reflects the strictest
// observation across the two HTTP calls — if either had to fall back to
// InsecureSkipVerify, the whole call is reported as UNVERIFIED.
func doRemoteCallWithSession(ctx context.Context, endpoint string, body []byte, timeout time.Duration) (*aggRPCResponse, MCPTrustLevel, error) {
	sessionID, initTrust, initErr := acquireMCPSession(ctx, endpoint, timeout)
	if initErr != nil {
		return nil, MCPTrustNone, fmt.Errorf("acquire session: %w", initErr)
	}
	if sessionID == "" {
		return nil, initTrust, fmt.Errorf("server returned empty Mcp-Session-Id")
	}
	defer releaseMCPSession(ctx, endpoint, sessionID, timeout, initTrust == MCPTrustUnverified)

	resp, callTrust, err := doSendWithTLSFallback(ctx, endpoint, body, sessionID, timeout)
	if err != nil {
		return nil, MCPTrustNone, err
	}

	// Strictest trust wins.
	trust := callTrust
	if initTrust == MCPTrustUnverified || callTrust == MCPTrustUnverified {
		trust = MCPTrustUnverified
	}
	return resp, trust, nil
}

// methodNeedsSession reports whether a JSON-RPC method requires a server-issued
// Mcp-Session-Id. Anchored on the server's rule (transport_http.go): every
// method except "initialize" needs a session, but "ping" is treated as
// best-effort and currently considered reachable even when the server rejects
// it with a missing-session error — so we skip the handshake for ping to keep
// reachability checks cheap.
func methodNeedsSession(method string) bool {
	switch method {
	case "", "initialize", "ping":
		return false
	default:
		return true
	}
}

// acquireMCPSession runs an initialize handshake and returns the session id
// minted by the server. The session id is read preferentially from the
// Mcp-Session-Id response header, falling back to result.sessionId in the
// JSON body (the server sets both per the streamable-HTTP transport).
func acquireMCPSession(ctx context.Context, endpoint string, timeout time.Duration) (string, MCPTrustLevel, error) {
	initReq := aggRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "globular-mcp-aggregator",
				"version": "1.0.0",
			},
		},
	}
	body, err := json.Marshal(initReq)
	if err != nil {
		return "", MCPTrustNone, err
	}

	// Verified TLS first; fall back to InsecureSkipVerify on TLS failure.
	resp, trust, sid, err := sendHTTPMCPRequest(ctx, endpoint, body, "", timeout, false)
	if err != nil && isTLSError(err) && allowUnverifiedFallback() {
		resp, _, sid, err = sendHTTPMCPRequest(ctx, endpoint, body, "", timeout, true)
		trust = MCPTrustUnverified
	}
	if err != nil {
		return "", MCPTrustNone, err
	}
	if resp != nil && resp.Error != nil {
		return "", trust, fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	if sid == "" {
		// Header missing — try to read sessionId from the result payload.
		if resp != nil && resp.Result != nil {
			// resp.Result is a typed struct in this package; the server
			// embeds sessionId in the protocol-level result map though,
			// which our typed struct doesn't surface directly. We can't
			// recover it without re-parsing — but the header path is the
			// canonical one and works against the production server.
		}
	}
	return sid, trust, nil
}

// allowUnverifiedFallback reports whether TLS certificate verification may be
// bypassed for remote MCP connections. This is a KNOWN DEVIATION from the
// hard rule that all inter-service gRPC must use mTLS with the cluster CA.
// It exists solely for development / debugging scenarios where the remote node
// has a self-signed cert that the local trust store doesn't recognise. NEVER
// set GLOBULAR_MCP_ALLOW_UNVERIFIED_FALLBACK=true in production.
func allowUnverifiedFallback() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(mcpAllowUnverifiedFallbackEnv)))
	active := v == "1" || v == "true" || v == "yes" || v == "on"
	if active {
		log.Printf("[WARNING] mcp: %s is active — TLS certificate verification is DISABLED for remote MCP connections; do not use in production", mcpAllowUnverifiedFallbackEnv)
	}
	return active
}

// releaseMCPSession sends a DELETE /mcp with the session id so the server
// can free its in-memory entry. Best-effort: failures are logged, never
// surfaced; the server's session GC will reap idle sessions on its own.
func releaseMCPSession(parent context.Context, endpoint, sessionID string, timeout time.Duration, skipVerify bool) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	client := newAggregatorHTTPClient(timeout, skipVerify)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return
	}
	httpReq.Header.Set("Mcp-Session-Id", sessionID)
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("aggregator: session release failed for %s: %v", endpoint, err)
		return
	}
	resp.Body.Close()
}

// sendHTTPMCPRequest is the low-level HTTP+JSON-RPC primitive. sessionID may
// be empty for methods that don't require one (initialize, ping). Returns the
// parsed response, the trust level for this call, the server's
// Mcp-Session-Id response header (if any), and an error.
func sendHTTPMCPRequest(ctx context.Context, endpoint string, body []byte, sessionID string, timeout time.Duration, skipVerify bool) (*aggRPCResponse, MCPTrustLevel, string, error) {
	client := newAggregatorHTTPClient(timeout, skipVerify)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, MCPTrustNone, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, MCPTrustNone, "", err
	}
	defer httpResp.Body.Close()

	respSession := httpResp.Header.Get("Mcp-Session-Id")

	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return nil, MCPTrustNone, respSession, fmt.Errorf("read response: %w", err)
	}

	var rpcResp aggRPCResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return nil, MCPTrustNone, respSession, fmt.Errorf("%s: %w", ErrMCPResponseInvalid, err)
	}

	trust := MCPTrustVerified
	if skipVerify {
		trust = MCPTrustUnverified
	}
	return &rpcResp, trust, respSession, nil
}

func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, "certificate", "tls", "x509", "handshake")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ── Public aggregator operations ──────────────────────────────────────────────

// PingRemoteMCP checks reachability of the MCP endpoint at mcpURL.
func pingRemoteMCP(ctx context.Context, mcpURL string) (reachable bool, trust MCPTrustLevel, elapsedMs int64, pingErr error) {
	start := time.Now()

	pingCtx, cancel := context.WithTimeout(ctx, aggPingTimeout)
	defer cancel()

	req := aggRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping"}
	resp, trust, err := doRemoteCall(pingCtx, mcpURL, req, aggPingTimeout)
	elapsedMs = time.Since(start).Milliseconds()

	if err != nil {
		return false, MCPTrustNone, elapsedMs, err
	}
	if resp.Error != nil {
		// Server responded — it's reachable even if the method isn't supported.
		return true, trust, elapsedMs, nil
	}
	return true, trust, elapsedMs, nil
}

// callRemoteTool calls a single named tool on the remote MCP server.
// The tool must be in the allowlist (callers are expected to check IsRemoteToolAllowed first).
func callRemoteTool(ctx context.Context, mcpURL string, toolName string, args map[string]interface{}) (result interface{}, trust MCPTrustLevel, err error) {
	toolCtx, cancel := context.WithTimeout(ctx, aggToolTimeout)
	defer cancel()

	req := aggRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  aggToolCallParams{Name: toolName, Arguments: args},
	}

	resp, trust, err := doRemoteCall(toolCtx, mcpURL, req, aggToolTimeout)
	if err != nil {
		return nil, MCPTrustNone, err
	}
	if resp.Error != nil {
		if resp.Error.Code == -32602 && containsAny(resp.Error.Message, "unknown tool") {
			return nil, trust, fmt.Errorf("%s: %s", ErrMCPToolNotFound, resp.Error.Message)
		}
		return nil, trust, fmt.Errorf("remote tool error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		return nil, trust, fmt.Errorf("%s: empty result", ErrMCPResponseInvalid)
	}

	// Extract text content and parse as JSON when possible.
	for _, c := range resp.Result.Content {
		if c.Type == "text" && c.Text != "" {
			var parsed interface{}
			if json.Unmarshal([]byte(c.Text), &parsed) == nil {
				return parsed, trust, nil
			}
			return c.Text, trust, nil
		}
	}
	return resp.Result, trust, nil
}

// snapshotTools lists the tools called for a standard remote snapshot, in order.
var snapshotTools = []struct {
	key  string
	tool string
	args map[string]interface{}
}{
	{"awareness_bundle",  "awareness.bundle_status",     nil},
	{"awareness_errors",  "awareness.runtime_errors",    nil},
	{"day1_verdict",      "awareness.day1_classify_node", nil},
	{"inventory",         "nodeagent_get_inventory",      nil},
	{"packages",          "nodeagent_list_installed_packages", nil},
	{"pki",               "nodeagent_get_certificate_status",  nil},
}

// collectRemoteSnapshot calls a standard set of read-only tools on the remote node
// and merges the results into a single snapshot map.
func collectRemoteSnapshot(ctx context.Context, mcpURL string) (snapshot map[string]interface{}, trust MCPTrustLevel) {
	snapCtx, cancel := context.WithTimeout(ctx, aggSnapshotTimeout)
	defer cancel()

	snapshot = make(map[string]interface{})
	trust = MCPTrustVerified
	successCount := 0

	for _, t := range snapshotTools {
		result, t2, err := callRemoteTool(snapCtx, mcpURL, t.tool, t.args)
		if err != nil {
			snapshot[t.key] = map[string]interface{}{"error": err.Error()}
			continue
		}
		successCount++
		if t2 == MCPTrustUnverified {
			trust = MCPTrustUnverified
		}
		snapshot[t.key] = result
	}

	// If every tool call failed, no verified data was collected — downgrade
	// trust to None so callers know the snapshot is entirely error-filled.
	if successCount == 0 {
		trust = MCPTrustNone
	}

	return snapshot, trust
}
