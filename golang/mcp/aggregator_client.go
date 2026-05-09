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
// InsecureSkipVerify and reports MCPTrustUnverified. This matches the phase-1
// contract: "explicitly report the trust level rather than silently failing."
func doRemoteCall(ctx context.Context, mcpURL string, req aggRPCRequest, timeout time.Duration) (resp *aggRPCResponse, trust MCPTrustLevel, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, MCPTrustNone, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := mcpURL + "/mcp"

	// Attempt 1: verified TLS (cluster CA).
	resp, trust, err = sendHTTPMCPRequest(ctx, endpoint, body, timeout, false)
	if err == nil {
		return resp, trust, nil
	}

	// On TLS error, retry with InsecureSkipVerify and report unverified trust.
	if isTLSError(err) {
		log.Printf("aggregator: TLS verification failed for %s: %v — retrying unverified", mcpURL, err)
		resp, _, err = sendHTTPMCPRequest(ctx, endpoint, body, timeout, true)
		if err == nil {
			return resp, MCPTrustUnverified, nil
		}
	}

	return nil, MCPTrustNone, err
}

func sendHTTPMCPRequest(ctx context.Context, endpoint string, body []byte, timeout time.Duration, skipVerify bool) (*aggRPCResponse, MCPTrustLevel, error) {
	client := newAggregatorHTTPClient(timeout, skipVerify)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, MCPTrustNone, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, MCPTrustNone, err
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return nil, MCPTrustNone, fmt.Errorf("read response: %w", err)
	}

	var rpcResp aggRPCResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return nil, MCPTrustNone, fmt.Errorf("%s: %w", ErrMCPResponseInvalid, err)
	}

	trust := MCPTrustVerified
	if skipVerify {
		trust = MCPTrustUnverified
	}
	return &rpcResp, trust, nil
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

	for _, t := range snapshotTools {
		result, t2, err := callRemoteTool(snapCtx, mcpURL, t.tool, t.args)
		if err != nil {
			snapshot[t.key] = map[string]interface{}{"error": err.Error()}
			continue
		}
		if t2 == MCPTrustUnverified {
			trust = MCPTrustUnverified
		}
		snapshot[t.key] = result
	}
	return snapshot, trust
}
