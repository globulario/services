package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// ── JSON-RPC 2.0 types ─────────────────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

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

// ── MCP protocol types ──────────────────────────────────────────────────────

type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]propSchema  `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

type propSchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type toolResultContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolResultContent `json:"content"`
	IsError bool                `json:"isError,omitempty"`
}

// ── Tool handler ────────────────────────────────────────────────────────────

// toolHandler is a function that executes a tool and returns a JSON-serializable result.
type toolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

type registeredTool struct {
	def     toolDef
	handler toolHandler
}

// ── Server ──────────────────────────────────────────────────────────────────

type server struct {
	mu      sync.RWMutex
	tools   map[string]*registeredTool
	order   []string // insertion order for listing
	clients *clientPool
	cfg     *MCPConfig
	sem     chan struct{} // concurrency limiter
}

func newServer(cfg *MCPConfig) *server {
	limit := cfg.ConcurrencyLimit
	if limit <= 0 {
		limit = 10
	}
	return &server{
		tools:   make(map[string]*registeredTool),
		clients: newClientPool(),
		cfg:     cfg,
		sem:     make(chan struct{}, limit),
	}
}

func (s *server) register(def toolDef, handler toolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[def.Name] = &registeredTool{def: def, handler: handler}
	s.order = append(s.order, def.Name)
}

func (s *server) serveStdio(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stdin: %w", err)
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("invalid JSON-RPC: %v", err)
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				log.Printf("write response: %v", err)
			}
		}
	}
}

func (s *server) handleRequest(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil // notification, no response
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *server) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "globular-mcp-server",
				"version": "0.1.0",
			},
		},
	}
}

func (s *server) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
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

func (s *server) handleToolsCall(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: "invalid tool call params"},
		}
	}

	s.mu.RLock()
	tool, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
		}
	}

	// Acquire concurrency slot.
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return &jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32000, Message: "server busy"},
		}
	}

	start := time.Now()
	result, err := tool.handler(ctx, params.Arguments)
	if s.cfg.AuditLog {
		auditLog(ctx, params.Name, params.Arguments, start, err)
	}
	if err != nil {
		errText := translateError(err)
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolResult{
				Content: []toolResultContent{{Type: "text", Text: errText}},
				IsError: true,
			},
		}
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	if s.cfg.MaxResponseSize > 0 && len(data) > s.cfg.MaxResponseSize {
		data = append(data[:s.cfg.MaxResponseSize], []byte("\n... (truncated)")...)
	}
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolResult{
			Content: []toolResultContent{{Type: "text", Text: string(data)}},
		},
	}
}
