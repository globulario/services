package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/interceptors"
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
	Type       string                `json:"type"`
	Properties map[string]propSchema `json:"properties,omitempty"`
	Required   []string              `json:"required,omitempty"`
}

type propSchema struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type toolResultContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // base64 for image type
	MimeType string `json:"mimeType,omitempty"` // e.g. "image/png"
}

type toolResult struct {
	Content []toolResultContent `json:"content"`
	IsError bool                `json:"isError,omitempty"`
}

// imageToolResult is returned by tool handlers that produce image data.
// The server detects this type and formats it as an MCP image content block.
type imageToolResult struct {
	Data     string // base64-encoded image data
	MimeType string // e.g. "image/png"
	Text     string // optional text description alongside the image
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
	writer := bufio.NewWriter(os.Stdout)

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
			return fmt.Errorf("read stdin: %w", err)
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("invalid JSON-RPC: %v", err)
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(data)); err == nil {
				_, err = writer.Write(data)
			}
			if err != nil {
				log.Printf("write response: %v", err)
			}
			writer.Flush()
		}
	}
}

// readStdioMessage reads a single MCP message from a bufio.Reader. It supports
// both the official MCP framing (Content-Length headers like LSP) and a
// best-effort fallback for newline-delimited JSON used by early prototypes.
func readStdioMessage(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")

		// Skip leading blank lines that some clients send between messages.
		if trimmed == "" {
			continue
		}

		// Newline-delimited JSON fallback: if the line starts with '{', treat it
		// as the whole message.
		if strings.HasPrefix(trimmed, "{") {
			return []byte(trimmed), nil
		}

		// Otherwise parse LSP-style headers until the blank line, then read the
		// declared Content-Length bytes for the body.
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
				break // end of headers
			}
		}

		cl := headers["content-length"]
		if cl == "" {
			return nil, fmt.Errorf("missing Content-Length header")
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

func (s *server) handleRequest(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil // notification, no response
	case "notifications/initialized":
		return nil // MCP streamable HTTP notification, no response
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "prompts/list":
		return s.handlePromptsList(req)
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
			// Use the latest published protocol version for maximum client compatibility.
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				// Claude Code / Codex expect explicit authentication advertising.
				// We only support unauthenticated local HTTP.
				"authentication": map[string]interface{}{
					"methods":  []string{"none"},
					"required": false,
				},
				// Discovery features explicitly declared to avoid clients assuming “unsupported”.
				"tools": map[string]interface{}{
					"listChanged": false,
				},
				"resources": map[string]interface{}{
					"listChanged": false,
					"subscribe":   false,
				},
				"prompts": map[string]interface{}{
					"listChanged": false,
				},
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

// handleResourcesList returns an empty resource list so clients that probe
// resources do not treat the method as unsupported.
func (s *server) handleResourcesList(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"resources": []interface{}{}},
	}
}

// handlePromptsList returns an empty prompt list to satisfy MCP discovery calls.
func (s *server) handlePromptsList(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"prompts": []interface{}{}},
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
	duration := time.Since(start)
	if s.cfg.AuditLog {
		auditLog(ctx, params.Name, params.Arguments, start, err)
	}

	// Emit to interceptor ring buffer for AI-queryable structured logs.
	{
		code := "OK"
		level := "TRACE"
		msg := "tool call: " + params.Name
		if err != nil {
			code = "ERROR"
			level = "WARN"
			msg = "tool error: " + params.Name + ": " + err.Error()
		}
		interceptors.EmitLog(level, "mcp", params.Name, "", "", code, msg, duration.Milliseconds(), nil)
	}

	if err != nil {
		// Invalidate cached connections on TLS or connectivity errors so
		// the next call re-dials with fresh credentials. This handles
		// cert rotation and cluster reinstalls without requiring a restart.
		if isConnError(err) {
			s.clients.close()
		}
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

	// Also check successful results for embedded connectivity errors
	// (composed tools return partial results with error lists).
	if m, ok := result.(map[string]interface{}); ok {
		if errs, ok := m["errors"].([]string); ok {
			for _, e := range errs {
				if isConnError(fmt.Errorf("%s", e)) {
					s.clients.close()
					break
				}
			}
		}
	}

	// If the handler returned an image result, format as MCP image content block.
	if img, ok := result.(*imageToolResult); ok {
		content := []toolResultContent{
			{Type: "image", Data: img.Data, MimeType: img.MimeType},
		}
		if img.Text != "" {
			content = append(content, toolResultContent{Type: "text", Text: img.Text})
		}
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  toolResult{Content: content},
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
