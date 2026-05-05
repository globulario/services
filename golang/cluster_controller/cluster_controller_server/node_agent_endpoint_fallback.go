package main

import (
	"fmt"
	"net"
	"strings"

	"google.golang.org/grpc"
)

// fallbackNodeAgentEndpointFromState derives a direct-IP endpoint for the node
// when the registered agent endpoint host is DNS-based or unavailable.
// Returns empty string when no safe fallback can be computed.
func (srv *server) fallbackNodeAgentEndpointFromState(nodeID, current string) string {
	if nodeID == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(current))
	if err != nil || port == "" {
		return ""
	}
	// If host is already an IP, no fallback needed.
	if ip := net.ParseIP(host); ip != nil {
		return ""
	}

	srv.lock("fallbackNodeAgentEndpointFromState")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return ""
	}

	candidates := []string{
		strings.TrimSpace(node.PrimaryIP()),
	}
	for _, ip := range node.Identity.Ips {
		candidates = append(candidates, strings.TrimSpace(ip))
	}
	for _, c := range candidates {
		ip := net.ParseIP(c)
		if ip == nil || ip.IsLoopback() || ip.To4() == nil {
			continue
		}
		return net.JoinHostPort(c, port)
	}
	return ""
}

// nodeIDForEndpoint returns the node ID whose AgentEndpoint matches the given
// endpoint string. Used when the caller only has the endpoint and needs to
// enable the fallback dial path. Returns "" if not found.
func (srv *server) nodeIDForEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	srv.lock("nodeIDForEndpoint")
	defer srv.unlock()
	if srv.state == nil {
		return ""
	}
	for id, n := range srv.state.Nodes {
		if n != nil && strings.TrimSpace(n.AgentEndpoint) == endpoint {
			return id
		}
	}
	return ""
}

// dialNodeAgentForEndpoint dials with fallback using endpoint-to-nodeID lookup.
// Use this when you have an endpoint but not a nodeID.
func (srv *server) dialNodeAgentForEndpoint(endpoint string) (*grpc.ClientConn, string, error) {
	nodeID := srv.nodeIDForEndpoint(endpoint)
	return srv.dialNodeAgentForNode(nodeID, endpoint)
}

// dialNodeAgentForNode dials the preferred endpoint first, then retries with a
// direct-IP fallback derived from controller node state when available.
func (srv *server) dialNodeAgentForNode(nodeID, preferred string) (*grpc.ClientConn, string, error) {
	conn, err := srv.dialNodeAgent(preferred)
	if err == nil {
		return conn, preferred, nil
	}
	fallback := srv.fallbackNodeAgentEndpointFromState(nodeID, preferred)
	if fallback == "" || fallback == preferred {
		return nil, preferred, err
	}
	conn2, err2 := srv.dialNodeAgent(fallback)
	if err2 != nil {
		return nil, preferred, fmt.Errorf("dial preferred failed: %w; fallback %s failed: %v", err, fallback, err2)
	}
	return conn2, fallback, nil
}
