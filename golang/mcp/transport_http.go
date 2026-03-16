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
	"strings"
	"time"
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
