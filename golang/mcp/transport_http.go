package main

import (
	"context"
	"crypto/rand"
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
)

// sessionStore tracks active MCP sessions for the Streamable HTTP transport.
var (
	sessionMu   sync.RWMutex
	sessionSet  = map[string]bool{} // valid session IDs
)

// serveHTTP starts an HTTP server that accepts JSON-RPC MCP requests via POST.
// This is the cluster-facing transport for remote MCP clients (via Envoy).
func (s *server) serveHTTP(ctx context.Context, listenAddr string) error {
	mux := http.NewServeMux()

	// MCP endpoint: POST /mcp with JSON-RPC body.
	// Responds using SSE (text/event-stream) for MCP Streamable HTTP transport
	// compatibility, or plain JSON if the client doesn't accept SSE.
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
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
				sessionMu.Lock()
				delete(sessionSet, sid)
				sessionMu.Unlock()
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
			return
		}
		defer r.Body.Close()

		var req jsonRPCRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &jsonRPCError{Code: -32700, Message: "parse error"},
			})
			return
		}

		// For non-initialize requests, validate the session ID if provided.
		if req.Method != "initialize" {
			sid := r.Header.Get("Mcp-Session-Id")
			if sid != "" {
				sessionMu.RLock()
				valid := sessionSet[sid]
				sessionMu.RUnlock()
				if !valid {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(jsonRPCResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Error:   &jsonRPCError{Code: -32600, Message: "invalid or expired session"},
					})
					return
				}
			}
		}

		// Inject caller identity from token header into context for audit logging.
		reqCtx := r.Context()
		if token := r.Header.Get("token"); token != "" {
			// Extract caller from token (best-effort, don't block on failure).
			if caller := extractCallerFromToken(token); caller != "" {
				reqCtx = context.WithValue(reqCtx, callerKey, caller)
			}
		}

		resp := s.handleRequest(reqCtx, &req)

		// Check if the client accepts SSE (MCP Streamable HTTP transport).
		accept := r.Header.Get("Accept")
		wantSSE := strings.Contains(accept, "text/event-stream")

		// For initialize responses, generate and attach session ID.
		if req.Method == "initialize" && resp != nil && resp.Error == nil {
			sid := generateSessionID()
			sessionMu.Lock()
			sessionSet[sid] = true
			sessionMu.Unlock()
			w.Header().Set("Mcp-Session-Id", sid)
		}

		if resp == nil {
			// Notification — no response needed.
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if wantSSE {
			// Respond in SSE format for MCP Streamable HTTP compatibility.
			data, _ := json.Marshal(resp)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		} else {
			// Plain JSON fallback (curl, legacy clients).
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
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
		ln      net.Listener
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
	log.Printf("globular-mcp-server: HTTP listening on %s", ln.Addr())

	// Do NOT overwrite the configured port — the config should be the
	// source of truth. Random fallback ports caused instability.

	// Update .mcp.json files so Claude Code can reconnect without a restart.
	updateMCPJsonFiles(actualPort)

	// MCP is a local-only HTTP service (like gateway/xds). It is NOT a gRPC
	// service and should not appear in the Service Instances table. Claude Code
	// connects directly to 127.0.0.1:<port>; no Envoy routing is needed.

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
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
func updateMCPJsonFiles(actualPort int) {
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
	ensuredPaths := ensureClaudeMCPConfigs(actualPort)
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
		patchMCPJson(abs, actualPort)
	}
}

// ensureClaudeMCPConfigs ensures ~/.claude/.mcp.json exists in all user home
// directories under /home/ (and /root/). If the file doesn't exist, it creates
// it with the correct MCP server URL. Returns paths that were created or
// already existed for further patching.
func ensureClaudeMCPConfigs(port int) []string {
	var paths []string

	homeDirs := []string{"/root", "/var/lib/globular"}
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				homeDirs = append(homeDirs, filepath.Join("/home", e.Name()))
			}
		}
	}

	mcpJSON := fmt.Sprintf(`{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "http://127.0.0.1:%d/mcp"
    }
  }
}
`, port)

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

		// Fix ownership: match the home directory owner so Claude Code
		// (running as the user) can read/write the file.
		if info, err := os.Stat(home); err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				_ = os.Chown(claudeDir, int(stat.Uid), int(stat.Gid))
				_ = os.Chown(mcpPath, int(stat.Uid), int(stat.Gid))
			}
		}

		log.Printf("mcp: created %s (port %d)", mcpPath, port)
		paths = append(paths, mcpPath)
	}

	return paths
}

// patchMCPJson reads a .mcp.json file, updates any localhost MCP server URL to
// use the given port, and writes it back if changed.
func patchMCPJson(path string, port int) {
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
		if !strings.Contains(urlStr, "localhost") && !strings.Contains(urlStr, "127.0.0.1") {
			continue
		}
		if !strings.Contains(urlStr, "/mcp") {
			continue
		}

		// Build the new URL with the actual port.
		newURL := fmt.Sprintf("http://localhost:%d/mcp", port)
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
