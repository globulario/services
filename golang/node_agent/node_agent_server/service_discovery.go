package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// discoverServiceAddr returns the address for a gRPC service.
//
// On Day 0 / control-plane nodes where the service runs locally, it returns
// localhost:<port>. On Day 1 joining nodes where the service isn't local,
// it returns the gateway address (<controller-host>:8443). Envoy on the
// gateway routes gRPC by service path (e.g. /event.EventService/* →
// event_EventService_cluster) so any service is reachable through one address.
//
// This replaces all hardcoded localhost:<port> addresses in the node-agent.
func discoverServiceAddr(defaultLocalPort int) string {
	localAddr := fmt.Sprintf("localhost:%d", defaultLocalPort)

	// Fast check: is the service running locally?
	if isLocalPortOpen(defaultLocalPort) {
		return localAddr
	}

	// Not local — route through the gateway.
	if gw := discoverGatewayAddr(); gw != "" {
		return gw
	}

	// Last resort — return localhost and let the caller handle the error.
	return localAddr
}

// discoverGatewayAddr returns <controller-host>:8443 for remote service access.
// The gateway (Envoy) proxies all gRPC traffic to backend services based on the
// service path prefix, so one address handles every service.
//
// Discovery order:
//  1. Cached value (gateway doesn't move during a session)
//  2. Node-agent state file (controller_endpoint → same host, port 8443)
//  3. NODE_AGENT_CONTROLLER_ENDPOINT env var
//  4. DNS: controller.<domain>:8443
func discoverGatewayAddr() string {
	gatewayCache.mu.RLock()
	if gatewayCache.addr != "" {
		defer gatewayCache.mu.RUnlock()
		return gatewayCache.addr
	}
	gatewayCache.mu.RUnlock()

	addr := resolveGatewayAddr()
	if addr != "" {
		gatewayCache.mu.Lock()
		gatewayCache.addr = addr
		gatewayCache.mu.Unlock()
	}
	return addr
}

var gatewayCache struct {
	mu   sync.RWMutex
	addr string
}

func resolveGatewayAddr() string {
	// 1. State file
	stateRoot := os.Getenv("GLOBULAR_STATE_DIR")
	if stateRoot == "" {
		stateRoot = "/var/lib/globular"
	}
	// Try both possible state file locations.
	for _, rel := range []string{"node_agent/state.json", "nodeagent/state.json"} {
		statePath := stateRoot + "/" + rel
		if data, err := os.ReadFile(statePath); err == nil {
			var state struct {
				ControllerEndpoint string `json:"controller_endpoint"`
			}
			if json.Unmarshal(data, &state) == nil && state.ControllerEndpoint != "" {
				if host := hostFromEndpoint(state.ControllerEndpoint); host != "" {
					return net.JoinHostPort(host, "8443")
				}
			}
		}
	}

	// 2. Env var
	if ep := strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT")); ep != "" {
		if host := hostFromEndpoint(ep); host != "" {
			return net.JoinHostPort(host, "8443")
		}
	}

	// 3. DNS-based: try controller.<domain>:8443
	domain := strings.TrimSpace(os.Getenv("GLOBULAR_DOMAIN"))
	if domain == "" {
		domain = "globular.internal"
	}
	candidate := fmt.Sprintf("controller.%s:8443", domain)
	if host := hostFromEndpoint(candidate); host != "" {
		// Verify DNS resolves.
		if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
			return candidate
		}
	}

	return ""
}

func hostFromEndpoint(ep string) string {
	host, _, err := net.SplitHostPort(ep)
	if err != nil {
		return ""
	}
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "" // loopback doesn't help for remote discovery
	}
	return host
}

func isLocalPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
