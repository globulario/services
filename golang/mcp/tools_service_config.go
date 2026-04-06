package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func registerServiceConfigTools(s *server) {

	// ── service_config_get ──────────────────────────────────────────────────
	s.register(toolDef{
		Name: "service_config_get",
		Description: `Read a service's runtime configuration and state from etcd. Accepts a friendly name (e.g. "dns", "rbac", "event") or the full gRPC service name.

Returns the service's config (address, port, TLS, version) and runtime state (PID, state, last error). This is the live config from etcd — not what's on disk.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service_name": {Type: "string", Description: "Service name — friendly (e.g. 'dns', 'rbac', 'event', 'authentication') or full gRPC name (e.g. 'dns.DnsService')"},
			},
			Required: []string{"service_name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := strings.ToLower(strings.TrimSpace(getStr(args, "service_name")))
		if name == "" {
			return nil, fmt.Errorf("service_name is required")
		}

		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// List all service config keys
		resp, err := cli.Get(callCtx, "/globular/services/", clientv3.WithPrefix())
		if err != nil {
			return nil, fmt.Errorf("etcd list services: %w", err)
		}

		// Build a map of service_id -> {config, runtime}
		//go:schemalint:ignore — implementation type, not schema owner
		type svcEntry struct {
			id      string
			config  map[string]interface{}
			runtime map[string]interface{}
		}
		entries := make(map[string]*svcEntry)

		for _, kv := range resp.Kvs {
			key := string(kv.Key)
			rest := strings.TrimPrefix(key, "/globular/services/")
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) != 2 {
				continue
			}
			svcID, suffix := parts[0], parts[1]

			if entries[svcID] == nil {
				entries[svcID] = &svcEntry{id: svcID}
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(kv.Value, &parsed); err != nil {
				continue
			}

			switch suffix {
			case "config":
				entries[svcID].config = parsed
			case "runtime":
				entries[svcID].runtime = parsed
			}
		}

		// Find the matching service by friendly name or full gRPC name
		for _, entry := range entries {
			if entry.config == nil {
				continue
			}

			// Extract identifiers to match against
			svcName := strings.ToLower(fmt.Sprintf("%v", entry.config["Name"]))
			svcID := strings.ToLower(fmt.Sprintf("%v", entry.config["Id"]))

			// Match by: exact ID, exact name, or partial match on package prefix
			// e.g. "dns" matches "dns.DnsService", "24c70f5b-..."
			matched := false
			if strings.ToLower(entry.id) == name {
				matched = true
			} else if svcName == name || svcID == name {
				matched = true
			} else if strings.HasPrefix(svcName, name+".") || strings.HasPrefix(svcID, name+".") {
				matched = true
			} else if strings.Contains(svcName, name) {
				matched = true
			}

			if matched {
				result := map[string]interface{}{
					"service_id": entry.id,
					"config":     entry.config,
				}
				if entry.runtime != nil {
					result["runtime"] = entry.runtime
				}
				return result, nil
			}
		}

		return map[string]interface{}{
			"error": fmt.Sprintf("service %q not found in etcd", name),
		}, nil
	})

	// ── service_config_list ─────────────────────────────────────────────────
	s.register(toolDef{
		Name: "service_config_list",
		Description: `List all services registered in etcd with their name, port, state, and version. Lightweight overview — use service_config_get for full details.`,
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

		//go:schemalint:ignore — implementation type, not schema owner
		type svcSummary struct {
			config  map[string]interface{}
			runtime map[string]interface{}
		}
		byID := make(map[string]*svcSummary)

		for _, kv := range resp.Kvs {
			key := string(kv.Key)
			rest := strings.TrimPrefix(key, "/globular/services/")
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) != 2 {
				continue
			}
			svcID, suffix := parts[0], parts[1]

			if byID[svcID] == nil {
				byID[svcID] = &svcSummary{}
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(kv.Value, &parsed); err != nil {
				continue
			}

			switch suffix {
			case "config":
				byID[svcID].config = parsed
			case "runtime":
				byID[svcID].runtime = parsed
			}
		}

		services := make([]map[string]interface{}, 0, len(byID))
		for id, entry := range byID {
			if entry.config == nil {
				continue
			}
			svc := map[string]interface{}{
				"id":      id,
				"name":    entry.config["Name"],
				"port":    entry.config["Port"],
				"version": entry.config["Version"],
				"tls":     entry.config["TLS"],
			}
			if entry.runtime != nil {
				svc["state"] = entry.runtime["State"]
				svc["pid"] = entry.runtime["Process"]
			}
			services = append(services, svc)
		}

		return map[string]interface{}{
			"count":    len(services),
			"services": services,
		}, nil
	})
}
