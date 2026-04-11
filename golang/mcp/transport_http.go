package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/globulario/services/golang/config"
)

// sessionStore tracks active MCP HTTP sessions keyed by Mcp-Session-Id.
type mcpSession struct {
	createdAt time.Time
	lastSeen  time.Time
}

var (
	sessionMu    sync.RWMutex
	sessionStore = map[string]*mcpSession{}
)

func createSession() string {
	sid := generateSessionID()
	now := time.Now()
	sessionMu.Lock()
	sessionStore[sid] = &mcpSession{createdAt: now, lastSeen: now}
	sessionMu.Unlock()
	return sid
}

func deleteSession(id string) {
	sessionMu.Lock()
	delete(sessionStore, id)
	sessionMu.Unlock()
}

// touchSession validates a session id and updates its lastSeen timestamp.
func touchSession(id string) bool {
	sessionMu.Lock()
	s, ok := sessionStore[id]
	if ok {
		s.lastSeen = time.Now()
	}
	sessionMu.Unlock()
	return ok
}

// serveHTTP starts an HTTP server that accepts JSON-RPC MCP requests via POST.
// This is the cluster-facing transport for remote MCP clients (via Envoy).
func (s *server) serveHTTP(ctx context.Context, listenAddr string) error {
	mux := http.NewServeMux()

	// MCP endpoint: POST /mcp with JSON-RPC body.
	// Responds with JSON for MCP HTTP transport. GET /mcp can optionally open
	// an SSE stream for server-initiated notifications.
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// CORS + preflight support so browser-based MCP clients can connect.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, Authorization, token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		log.Printf("mcp: %s /mcp Accept=%q Mcp-Session-Id=%q", r.Method, r.Header.Get("Accept"), r.Header.Get("Mcp-Session-Id"))
		if r.Method == http.MethodGet {
			// GET with Accept: text/event-stream opens an SSE stream for
			// server-initiated notifications (MCP Streamable HTTP spec).
			// We don't send notifications, so just hold the connection open
			// until the client disconnects.
			if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			// Block until client disconnects.
			<-r.Context().Done()
			return
		}

		if r.Method == http.MethodDelete {
			// DELETE terminates a session.
			sid := r.Header.Get("Mcp-Session-Id")
			if sid != "" {
				deleteSession(sid)
				log.Printf("mcp: DELETE session %q", sid)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			log.Printf("mcp: error reading body: %v", err)
			return
		}
		defer r.Body.Close()

		trimmed := bytes.TrimSpace(body)
		if len(trimmed) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			log.Printf("mcp: empty POST body")
			return
		}

		sid := r.Header.Get("Mcp-Session-Id")

		// Parse single request or batch per MCP Streamable HTTP spec.
		var (
			requests   []jsonRPCRequest
			isBatch    bool
			parseError bool
		)

		if trimmed[0] == '[' {
			isBatch = true
			var rawBatch []json.RawMessage
			if err := json.Unmarshal(trimmed, &rawBatch); err != nil {
				parseError = true
			} else {
				for _, raw := range rawBatch {
					var req jsonRPCRequest
					if err := json.Unmarshal(raw, &req); err != nil {
						parseError = true
						break
					}
					requests = append(requests, req)
				}
			}
		} else {
			var req jsonRPCRequest
			if err := json.Unmarshal(trimmed, &req); err != nil {
				parseError = true
			} else {
				requests = append(requests, req)
			}
		}

		if parseError {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			payload := jsonRPCResponse{JSONRPC: "2.0", Error: &jsonRPCError{Code: -32700, Message: "parse error"}}
			json.NewEncoder(w).Encode(payload)
			log.Printf("mcp: parse error sid=%q", sid)
			return
		}

		var (
			responses    []jsonRPCResponse
			newSessionID string
		)

		for idx, req := range requests {
			msgType := "notification"
			if len(req.ID) > 0 {
				msgType = "request"
			}
			log.Printf("mcp: POST /mcp msg=%d method=%q type=%s sid=%q", idx, req.Method, msgType, sid)

			// For non-initialize calls, require and validate the session ID.
			if req.Method != "initialize" {
				if sid == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32600, Message: "missing Mcp-Session-Id"}}
					json.NewEncoder(w).Encode(resp)
					log.Printf("mcp: missing session for method %q", req.Method)
					return
				}
				if !touchSession(sid) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusNotFound)
					resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32600, Message: "invalid or expired session"}}
					json.NewEncoder(w).Encode(resp)
					log.Printf("mcp: invalid session %q for method %q", sid, req.Method)
					return
				}
			}

			// Inject caller identity from token header into context for audit logging.
			reqCtx := r.Context()
			if token := r.Header.Get("token"); token != "" {
				if caller := extractCallerFromToken(token); caller != "" {
					reqCtx = context.WithValue(reqCtx, callerKey, caller)
				}
			}

			resp := s.handleRequest(reqCtx, &req)
			if req.Method == "initialized" || req.Method == "notifications/initialized" {
				log.Printf("mcp: received initialized notification sid=%q", sid)
			}

			// For initialize responses, generate and attach session ID once.
			if req.Method == "initialize" && resp != nil && resp.Error == nil {
				newSessionID = createSession()
				if result, ok := resp.Result.(map[string]interface{}); ok {
					result["sessionId"] = newSessionID
				}
			}

			// Notifications do not yield JSON-RPC responses.
			if len(req.ID) == 0 {
				continue
			}

			if resp == nil {
				continue
			}
			responses = append(responses, *resp)
		}

		// Attach session header if initialize succeeded.
		if newSessionID != "" {
			w.Header().Set("Mcp-Session-Id", newSessionID)
			log.Printf("mcp: initialize success new session %q", newSessionID)
		}

		if len(responses) == 0 {
			w.WriteHeader(http.StatusAccepted)
			log.Printf("mcp: response status=%d content-type=%q body-bytes=%d", http.StatusAccepted, w.Header().Get("Content-Type"), 0)
			return
		}

		// Encode responses (single or batch) as JSON.
		var payload []byte
		if isBatch {
			payload, _ = json.Marshal(responses)
		} else {
			payload, _ = json.Marshal(responses[0])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		log.Printf("mcp: response status=%d content-type=%q body-bytes=%d", http.StatusOK, w.Header().Get("Content-Type"), len(payload))
	})

	// Legacy POST streaming endpoint kept for backward compatibility with
	// clients that still send NDJSON over a single POST and expect SSE replies.
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, Authorization, token")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			http.Error(w, "expected Accept: text/event-stream", http.StatusNotAcceptable)
			return
		}
		log.Printf("mcp: legacy /sse stream Accept=%q Mcp-Session-Id=%q", r.Header.Get("Accept"), r.Header.Get("Mcp-Session-Id"))
		s.handleStreamablePost(w, r)
	})

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"tools":     len(s.tools),
			"read_only": s.cfg.ReadOnly,
		})
	})

	// Retry binding the configured port a few times — Envoy or another
	// service may still be releasing it during startup sequencing.
	var (
		ln        net.Listener
		listenErr error
	)
	for attempt := 0; attempt < 10; attempt++ {
		ln, listenErr = net.Listen("tcp", listenAddr)
		if listenErr == nil {
			break
		}
		log.Printf("mcp: port %s unavailable (%v), retrying in 3s (%d/10)", listenAddr, listenErr, attempt+1)
		time.Sleep(3 * time.Second)
	}
	if listenErr != nil {
		return fmt.Errorf("listen %s: %w (gave up after 10 retries)", listenAddr, listenErr)
	}

	readTimeout := s.cfg.HTTPReadTimeout.Duration
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout := s.cfg.HTTPWriteTimeout.Duration
	if writeTimeout == 0 {
		writeTimeout = 60 * time.Second
	}

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Resolve the actual port (useful when listenAddr is ":0" or fallback).
	actualPort := ln.Addr().(*net.TCPAddr).Port
	log.Printf("globular-mcp-server: HTTP listening on %s (cfg=%s)", ln.Addr(), listenAddr)

	// Do NOT overwrite the configured port — the config should be the
	// source of truth. Random fallback ports caused instability.

	// Update .mcp.json files so Claude Code can reconnect without a restart.
	advertiseHost := s.cfg.HTTPAdvertiseHost
	if advertiseHost == "" {
		advertiseHost = config.GetRoutableIPv4()
	}
	scheme := "http"
	if s.cfg.HTTPUseTLS {
		scheme = "https"
	}
	updateMCPJsonFiles(actualPort, scheme, advertiseHost)

	// MCP is typically local-only, but can be exposed cluster-wide when
	// configured with a non-loopback listen address or TLS termination.

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if s.cfg.HTTPUseTLS {
		if s.cfg.HTTPTLSCertFile == "" || s.cfg.HTTPTLSKeyFile == "" {
			return fmt.Errorf("http serve: TLS enabled but certificate or key path is empty")
		}
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		if err := srv.ServeTLS(ln, s.cfg.HTTPTLSCertFile, s.cfg.HTTPTLSKeyFile); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("https serve: %w", err)
		}
		return nil
	}

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

// handleStreamablePost processes an MCP Streamable HTTP POST request where the
// client keeps the connection open and exchanges NDJSON messages over a single
// HTTP stream, while the server responds using Server‑Sent Events (SSE).
// This matches the MCP Streamable HTTP transport used by Claude Code.
func (s *server) handleStreamablePost(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Stream newline-delimited JSON requests from the client.
	scanner := bufio.NewScanner(r.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) // allow up to 1MB per message

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &jsonRPCError{Code: -32700, Message: "parse error"},
			}
			data, _ := json.Marshal(&resp)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			continue
		}

		sid := r.Header.Get("Mcp-Session-Id")
		log.Printf("mcp: legacy stream POST method=%q sid=%q", req.Method, sid)

		// For non-initialize requests, validate the session ID if provided.
		if req.Method != "initialize" {
			if sid == "" {
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &jsonRPCError{Code: -32600, Message: "missing Mcp-Session-Id"},
				}
				data, _ := json.Marshal(&resp)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
				continue
			}

			if !touchSession(sid) {
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &jsonRPCError{Code: -32600, Message: "invalid or expired session"},
				}
				data, _ := json.Marshal(&resp)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
				log.Printf("mcp: legacy stream invalid session %q", sid)
				continue
			}
		}

		// Inject caller identity from token header into context for audit logging.
		reqCtx := r.Context()
		if token := r.Header.Get("token"); token != "" {
			if caller := extractCallerFromToken(token); caller != "" {
				reqCtx = context.WithValue(reqCtx, callerKey, caller)
			}
		}

		resp := s.handleRequest(reqCtx, &req)

		// For initialize responses, generate and attach session ID.
		if req.Method == "initialize" && resp != nil && resp.Error == nil {
			sid := createSession()
			w.Header().Set("Mcp-Session-Id", sid)
			if result, ok := resp.Result.(map[string]interface{}); ok {
				result["sessionId"] = sid
			}
		}

		if resp == nil {
			// Notification — no response needed.
			continue
		}

		data, err := json.Marshal(resp)
		if err != nil {
			log.Printf("mcp: failed to marshal response: %v", err)
			continue
		}

		// Send as SSE event body.
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("mcp: stream read error: %v", err)
	}
}

// generateSessionID creates a random session identifier for the MCP Streamable HTTP transport.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp-based ID if crypto/rand fails.
		return fmt.Sprintf("mcp-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// extractCallerFromToken decodes the JWT payload (without verifying) to
// extract the caller principal for audit logging. Returns "" on failure.
func extractCallerFromToken(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	// Pad base64url to standard base64.
	payload := parts[1]
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	var claims struct {
		Sub string `json:"sub"`
		ID  string `json:"id"`
	}
	if json.Unmarshal(data, &claims) != nil {
		return ""
	}
	if claims.Sub != "" {
		return claims.Sub
	}
	return claims.ID
}

// updateMCPJsonFiles walks common locations for .mcp.json files and updates the
// port in any that point to localhost/127.0.0.1 with a different port. This
// allows Claude Code to reconnect after the MCP server restarts on a new port
// without requiring a full reboot.
//
// It also ensures ~/.claude/.mcp.json exists for all user home directories
// so Claude Code can discover the MCP server without manual configuration.
func updateMCPJsonFiles(actualPort int, scheme, host string) {
	candidates := []string{}

	// Check the services repo root (where this binary typically lives).
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for {
			p := filepath.Join(dir, ".mcp.json")
			if _, err := os.Stat(p); err == nil {
				candidates = append(candidates, p)
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Also check the current working directory.
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, ".mcp.json")
		if _, err := os.Stat(p); err == nil {
			candidates = append(candidates, p)
		}
	}

	// Check home directory.
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".mcp.json")
		if _, err := os.Stat(p); err == nil {
			candidates = append(candidates, p)
		}
	}

	// Ensure ~/.claude/.mcp.json exists for all user home directories.
	// This allows Claude Code to discover the MCP server automatically.
	ensuredPaths := ensureClaudeMCPConfigs(actualPort, scheme, host)
	candidates = append(candidates, ensuredPaths...)

	// Deduplicate.
	seen := map[string]bool{}
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		patchMCPJson(abs, actualPort, scheme, host)
	}
}

// ensureClaudeMCPConfigs ensures ~/.claude/.mcp.json exists in all user home
// directories under /home/ (and /root/). If the file doesn't exist, it creates
// it with the correct MCP server URL. Returns paths that were created or
// already existed for further patching.
func ensureClaudeMCPConfigs(port int, scheme, host string) []string {
	var paths []string

	uid := os.Geteuid()
	homeDirs := []string{}

	// Always include the current user.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeDirs = append(homeDirs, home)
	}

	// Only attempt other users' homes (root, service accounts) when running as root.
	if uid == 0 {
		homeDirs = append(homeDirs, "/root", "/var/lib/globular")
		if entries, err := os.ReadDir("/home"); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					homeDirs = append(homeDirs, filepath.Join("/home", e.Name()))
				}
			}
		}
	}

	mcpJSON := fmt.Sprintf(`{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "%s://%s:%d/mcp"
    }
  }
}
`, scheme, host, port)

	for _, home := range homeDirs {
		claudeDir := filepath.Join(home, ".claude")
		mcpPath := filepath.Join(claudeDir, ".mcp.json")

		if _, err := os.Stat(mcpPath); err == nil {
			// File exists — add to patch candidates.
			paths = append(paths, mcpPath)
			continue
		}

		// Create ~/.claude/ if needed, preserving ownership of the home dir.
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			continue
		}
		if err := os.WriteFile(mcpPath, []byte(mcpJSON), 0644); err != nil {
			log.Printf("mcp: failed to create %s: %v", mcpPath, err)
			continue
		}

		// Fix ownership when running as root.
		if uid == 0 {
			if info, err := os.Stat(home); err == nil {
				if stat, ok := info.Sys().(*syscall.Stat_t); ok {
					_ = os.Chown(claudeDir, int(stat.Uid), int(stat.Gid))
					_ = os.Chown(mcpPath, int(stat.Uid), int(stat.Gid))
				}
			}
		}

		log.Printf("mcp: created %s (port %d)", mcpPath, port)
		paths = append(paths, mcpPath)
	}

	return paths
}

// patchMCPJson reads a .mcp.json file, updates any localhost MCP server URL to
// use the given port/scheme/host, and writes it back if changed.
func patchMCPJson(path string, port int, scheme, host string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return
	}

	servers, ok := doc["mcpServers"].(map[string]interface{})
	if !ok {
		return
	}

	changed := false
	for name, raw := range servers {
		srv, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		urlStr, ok := srv["url"].(string)
		if !ok {
			continue
		}
		// Only patch localhost URLs pointing to /mcp.
		if !strings.Contains(urlStr, "localhost") && !strings.Contains(urlStr, "127.0.0.1") && !strings.Contains(urlStr, "0.0.0.0") {
			continue
		}
		if !strings.Contains(urlStr, "/mcp") {
			continue
		}

		// Build the new URL with the actual port and routable IP.
		newURL := fmt.Sprintf("%s://%s:%d/mcp", scheme, host, port)
		if urlStr != newURL {
			srv["url"] = newURL
			changed = true
			log.Printf("mcp: updated %s server %q URL: %s → %s", path, name, urlStr, newURL)
		}
	}

	if !changed {
		return
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Printf("mcp: failed to marshal updated %s: %v", path, err)
		return
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0644); err != nil {
		log.Printf("mcp: failed to write updated %s: %v", path, err)
	}
}
