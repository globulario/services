// Package mcp implements a standalone MCP server that exposes the Globular
// awareness graph as tools for AI agents.
//
// Deprecated: The awareness tools are now exposed by the main Globular MCP
// service (golang/mcp). Use that service for all production deployments.
// This package remains for local development and testing only — it will be
// removed in a future release.
//
// Migration: Enable tool_groups.awareness=true in the main MCP service config
// (/var/lib/globular/mcp/config.json). The main service automatically includes
// all 12 awareness tools when the awareness graph DB is present.
//
// Protocol: JSON-RPC 2.0 over stdio with Content-Length framing (same as LSP).
// Transport: stdin / stdout.
//
// All tools are read-only except propose_from_incident, validate_proposal, and
// approve_proposal, which obey the existing approval/promotion rules.
// promote-proposal is intentionally NOT exposed over MCP.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/globulario/services/golang/awareness/graph"
)

// Config holds the awareness MCP server configuration.
type Config struct {
	DBPath   string // path to graph.db; empty → <repo>/.globular/awareness/graph.db
	RepoPath string // repo root; empty → auto-detect via git
	DocsDir  string // path to docs/awareness; empty → <repo>/docs/awareness
	NodeID   string // optional: local node ID for runtime bridge labelling
	// Cluster endpoints for runtime bridge gRPC sources.
	// All are optional — if empty, the source degrades gracefully.
	ControllerAddr string // e.g. "10.0.0.63:12000"
	DoctorAddr     string // e.g. "10.0.0.63:12005"
	WorkflowAddr   string // e.g. "10.0.0.63:10004"
	PrometheusAddr string // e.g. "http://10.0.0.63:9090"
	// TLS settings for gRPC runtime sources.
	// If Insecure is true, plain-text transport is used (local dev/test only).
	// For production, set CACert (and optionally ClientCert/ClientKey) for mTLS.
	Insecure   bool   // if true, use insecure transport (local dev/test only)
	CACert     string // path to CA cert PEM
	ClientCert string // path to client cert PEM
	ClientKey  string // path to client key PEM
	ServerName string // TLS server name override
}

// jsonRPCRequest is an incoming JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse is an outgoing JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// toolCallParams is the params field of a tools/call request.
type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// toolResultContent is a single content block in a tool result.
type toolResultContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// toolResult is the result field of a tools/call response.
type toolResult struct {
	Content []toolResultContent `json:"content"`
	IsError bool                `json:"isError,omitempty"`
}

// Server is the awareness MCP server.
type Server struct {
	mu    sync.RWMutex
	tools map[string]*registeredTool
	order []string // insertion order for listing
	cfg   Config
	g     *graph.Graph // may be nil if graph.db not found at startup
}

// New creates an awareness MCP server with all 12 tools registered.
// If the graph DB exists at the configured path it is opened immediately;
// otherwise the server starts in degraded mode and tools return warnings.
func New(cfg Config) *Server {
	s := &Server{
		tools: make(map[string]*registeredTool),
		cfg:   cfg,
	}

	// Try to open the graph (non-fatal — tools degrade gracefully).
	if dbPath := s.resolvedDBPath(); dbPath != "" {
		if g, err := graph.Open(dbPath); err == nil {
			s.g = g
		}
	}

	registerAllTools(s)
	return s
}

// NewWithGraph creates an awareness MCP server using a pre-opened graph.
// g may be nil — tools degrade gracefully.
// Use this in tests to supply an in-memory graph.
func NewWithGraph(cfg Config, g *graph.Graph) *Server {
	s := &Server{
		tools: make(map[string]*registeredTool),
		cfg:   cfg,
		g:     g,
	}
	registerAllTools(s)
	return s
}

// Close releases the graph DB handle.
func (s *Server) Close() {
	if s.g != nil {
		s.g.Close()
	}
}

// ServeRW runs the MCP server reading from r and writing to w until ctx is cancelled or EOF.
// Use this in tests to drive the server over io.Pipe instead of os.Stdin/Stdout.
func (s *Server) ServeRW(ctx context.Context, r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := readStdioMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("awareness-mcp: invalid JSON-RPC: %v", err)
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(data))
			writer.Write(data)
			writer.Flush()
		}
	}
}

// ServeStdio runs the MCP server on stdin/stdout until ctx is cancelled or EOF.
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.ServeRW(ctx, os.Stdin, os.Stdout)
}

// CallTool calls a tool by name with the given args and returns the raw result.
// This is used by tests and the CLI dry-run mode.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	s.mu.RLock()
	tool, ok := s.tools[name]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool.handler(ctx, args)
}

// ToolDef returns the definition of a registered tool, or nil if not found.
func (s *Server) ToolDef(name string) *ToolDefinition {
	s.mu.RLock()
	t, ok := s.tools[name]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return &ToolDefinition{
		Name:        t.def.Name,
		Description: t.def.Description,
		InputSchema: ToolInputSchema{
			Type:       t.def.InputSchema.Type,
			Required:   t.def.InputSchema.Required,
			Properties: len(t.def.InputSchema.Properties),
		},
	}
}

// ToolDefinition is the public view of a registered tool (for testing/inspection).
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema ToolInputSchema
}

// ToolInputSchema is the public view of a tool's input schema.
type ToolInputSchema struct {
	Type       string
	Required   []string
	Properties int // count of defined properties
}

// HasTool returns true if the named tool is registered.
func (s *Server) HasTool(name string) bool {
	s.mu.RLock()
	_, ok := s.tools[name]
	s.mu.RUnlock()
	return ok
}

// ToolNames returns the names of all registered tools in registration order.
func (s *Server) ToolNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

func (s *Server) handleRequest(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized", "notifications/initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"resources": []interface{}{}}}
	case "prompts/list":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"prompts": []interface{}{}}}
	case "ping":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func (s *Server) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{"listChanged": false},
				"resources": map[string]interface{}{"listChanged": false, "subscribe": false},
				"prompts":   map[string]interface{}{"listChanged": false},
			},
			"serverInfo": map[string]interface{}{
				"name":    "globular-awareness-mcp",
				"version": "1.0.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]toolDef, 0, len(s.order))
	for _, name := range s.order {
		if t, ok := s.tools[name]; ok {
			tools = append(tools, t.def)
		}
	}
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32602, Message: "invalid tool call params"},
		}
	}

	s.mu.RLock()
	tool, ok := s.tools[params.Name]
	s.mu.RUnlock()
	if !ok {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}

	result, err := tool.handler(ctx, params.Arguments)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolResult{
				Content: []toolResultContent{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
		}
	}

	text, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		text = []byte(`{"error":"marshal failed"}`)
	}
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolResult{
			Content: []toolResultContent{{Type: "text", Text: string(text)}},
		},
	}
}

// resolvedDBPath returns the graph DB path from config or the default location.
func (s *Server) resolvedDBPath() string {
	if s.cfg.DBPath != "" {
		return s.cfg.DBPath
	}
	repoRoot := s.resolvedRepoRoot()
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
}

// resolvedDocsDir returns the docs/awareness path.
func (s *Server) resolvedDocsDir() string {
	if s.cfg.DocsDir != "" {
		return s.cfg.DocsDir
	}
	repoRoot := s.resolvedRepoRoot()
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, "docs", "awareness")
}

// resolvedRepoRoot returns the repo root, auto-detecting via git if not configured.
func (s *Server) resolvedRepoRoot() string {
	if s.cfg.RepoPath != "" {
		return s.cfg.RepoPath
	}
	// Try git.
	out, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		// Fall back to cwd.
		cwd, _ := os.Getwd()
		return cwd
	}
	return strings.TrimSpace(out)
}

// readStdioMessage reads a single MCP message from the reader.
// Supports both Content-Length framing (LSP-style) and newline-delimited JSON.
func readStdioMessage(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			return []byte(trimmed), nil
		}

		headers := map[string]string{}
		for {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
			}
			line, err = r.ReadString('\n')
			if err != nil {
				return nil, err
			}
			trimmed = strings.TrimRight(line, "\r\n")
			if trimmed == "" {
				break
			}
		}

		cl := headers["content-length"]
		if cl == "" {
			return nil, fmt.Errorf("missing Content-Length")
		}
		n, err := strconv.Atoi(cl)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid Content-Length: %q", cl)
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf, nil
	}
}
