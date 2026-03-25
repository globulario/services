package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func registerFrontendTools(s *server) {

	// ── grpc_service_map ────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "grpc_service_map",
		Description: `List all running gRPC services with their ports and reachability status.

Shows each service's name, address, port, TLS status, and whether it is reachable via TCP.
Use this to find services that need Vite proxy entries or are down.`,
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		resp, err := cli.Get(callCtx, "/globular/services/", clientv3.WithPrefix())
		if err != nil {
			return nil, fmt.Errorf("etcd list services: %w", err)
		}

		services := make([]map[string]interface{}, 0)
		for _, kv := range resp.Kvs {
			var cfg map[string]interface{}
			if err := json.Unmarshal(kv.Value, &cfg); err != nil {
				continue
			}

			name, _ := cfg["Name"].(string)
			if name == "" {
				continue
			}

			addr, _ := cfg["Address"].(string)
			port, _ := cfg["Port"].(float64)
			tlsEnabled, _ := cfg["TLS"].(bool)
			proto, _ := cfg["Protocol"].(string)

			if addr == "" {
				addr = "localhost"
			}

			entry := map[string]interface{}{
				"name":            name,
				"address":         addr,
				"port":            int(port),
				"tls":             tlsEnabled,
				"protocol":        proto,
				"vite_proxy_path": "/" + name,
			}

			// Quick TCP reachability check
			endpoint := fmt.Sprintf("%s:%d", addr, int(port))
			conn, dialErr := net.DialTimeout("tcp", endpoint, 2*time.Second)
			if dialErr != nil {
				entry["reachable"] = false
				entry["error"] = dialErr.Error()
			} else {
				conn.Close()
				entry["reachable"] = true
			}

			services = append(services, entry)
		}

		return map[string]interface{}{
			"total":    len(services),
			"services": services,
		}, nil
	})

	// ── grpc_web_probe ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "grpc_web_probe",
		Description: `Test if a gRPC service is reachable through a gRPC-web proxy (Vite dev server or Envoy).

Sends an HTTP POST to the proxy endpoint and checks the response status.
- 200/415: proxy routes to the service (working)
- 404: proxy route missing (needs adding to vite.config.ts)
- 502/503: proxy exists but backend is down

Examples:
- grpc_web_probe(service="title.TitleService", proxy_url="http://localhost:5174")
- grpc_web_probe(service="media.MediaService", proxy_url="http://localhost:5173")`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service":   {Type: "string", Description: "gRPC service name (e.g. 'title.TitleService', 'media.MediaService')"},
				"proxy_url": {Type: "string", Description: "Proxy base URL (e.g. 'http://localhost:5174' for media app dev server)"},
				"method":    {Type: "string", Description: "Optional: specific method to probe (e.g. 'SearchTitles'). Default: empty POST to service path"},
			},
			Required: []string{"service", "proxy_url"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		service := getStr(args, "service")
		proxyURL := getStr(args, "proxy_url")
		method := getStr(args, "method")

		if service == "" || proxyURL == "" {
			return nil, fmt.Errorf("service and proxy_url are required")
		}

		proxyURL = strings.TrimSuffix(proxyURL, "/")
		path := "/" + service
		if method != "" {
			path += "/" + method
		}
		url := proxyURL + path

		httpClient := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(""))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/grpc-web+proto")

		resp, err := httpClient.Do(req)
		if err != nil {
			return map[string]interface{}{
				"service":   service,
				"proxy_url": proxyURL,
				"url":       url,
				"reachable": false,
				"error":     err.Error(),
				"diagnosis": "Proxy not running or URL incorrect",
			}, nil
		}
		defer resp.Body.Close()

		status := resp.StatusCode
		result := map[string]interface{}{
			"service":     service,
			"proxy_url":   proxyURL,
			"url":         url,
			"status_code": status,
		}

		switch {
		case status == 200:
			result["diagnosis"] = "OK — proxy routes to service and service responded"
			result["working"] = true
		case status == 415:
			result["diagnosis"] = "OK — proxy routes to service (415 = content-type mismatch, but route exists)"
			result["working"] = true
		case status == 404:
			result["diagnosis"] = "MISSING — proxy has no route for this service. Add to vite.config.ts proxy section"
			result["working"] = false
			result["fix"] = fmt.Sprintf(`Add to vite.config.ts proxy: '/%s': { target, changeOrigin: true, secure: false }`, service)
		case status == 502 || status == 503:
			result["diagnosis"] = "BACKEND DOWN — proxy route exists but the backend service is unreachable"
			result["working"] = false
		default:
			result["diagnosis"] = fmt.Sprintf("Unexpected status %d", status)
			result["working"] = false
		}

		return result, nil
	})
}
