package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

// serveHTTP starts an HTTP server that accepts JSON-RPC MCP requests via POST.
// This is the cluster-facing transport for remote MCP clients (via Envoy).
func (s *server) serveHTTP(ctx context.Context, listenAddr string) error {
	mux := http.NewServeMux()

	// MCP endpoint: POST /mcp with JSON-RPC body.
	// Responds using SSE (text/event-stream) for MCP Streamable HTTP transport
	// compatibility, or plain JSON if the client doesn't accept SSE.
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
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

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		// Port busy — fall back to OS-assigned free port so the server
		// stays reachable. The actual port is registered in etcd for discovery.
		log.Printf("mcp: port %s unavailable (%v), falling back to OS-assigned port", listenAddr, err)
		ln, err = net.Listen("tcp", ":0")
		if err != nil {
			return fmt.Errorf("listen fallback :0: %w", err)
		}
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

	// Persist the actual port back to config so it survives restarts and
	// is consistent with what gets registered in etcd.
	actualAddr := fmt.Sprintf(":%d", actualPort)
	if s.cfg.HTTPListenAddr != actualAddr {
		s.cfg.HTTPListenAddr = actualAddr
		writeDefaultConfig(s.cfg)
	}

	// Register in Globular service discovery so xDS/Envoy creates a route.
	// This lets Claude connect via https://<domain>/mcp through Envoy.
	registerMCPService(actualPort)

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

// registerMCPService registers the MCP server in Globular's service discovery
// (etcd) so the xDS watcher creates an Envoy cluster and subdomain route.
// This makes the MCP server accessible at https://mcp.<cluster-domain>/mcp.
// Runs in the background with retries so startup doesn't block or fail if
// etcd isn't ready yet (common during Day-0 bootstrap).
func registerMCPService(port int) {
	go func() {
		// MCP binds to localhost only — Envoy on the same host
		// terminates TLS and proxies to us.
		addr := "127.0.0.1"

		svcConfig := map[string]interface{}{
			"Id":       "mcp.MCPService",
			"Name":     "mcp.MCPService",
			"Address":  addr,
			"Port":     port,
			"Protocol": "http",
			"TLS":      false, // Envoy terminates TLS; MCP listens on plain HTTP
			"State":    "running",
			"Process":  os.Getpid(),
			"Version":  "0.0.1",
		}

		for attempt := 0; attempt < 10; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt*3) * time.Second)
			}
			if err := config.SaveServiceConfiguration(svcConfig); err != nil {
				log.Printf("mcp: service registration attempt %d/10 failed: %v", attempt+1, err)
				continue
			}
			log.Printf("mcp: registered as mcp.MCPService on %s:%d", addr, port)
			return
		}
		log.Printf("mcp: warning: service registration failed after 10 attempts; Envoy routing may not work")
	}()
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
