package config

import (
	"fmt"
	"strings"
)

// ClusterMode represents the operational mode of the Globular deployment.
type ClusterMode int

const (
	// ModeSingleNode is a development/bootstrap single-node deployment.
	// Loopback endpoints for cross-service traffic are acceptable.
	ModeSingleNode ClusterMode = iota

	// ModeCluster is a multi-node production deployment.
	// Loopback endpoints for cross-service traffic must be rejected.
	ModeCluster
)

// ValidateServiceEndpoint checks that a cross-service endpoint is valid for
// the given cluster mode. In ModeCluster, loopback addresses are rejected
// because cross-service traffic must be routable between nodes.
//
// Returns nil if the endpoint is valid, or an error describing the problem.
func ValidateServiceEndpoint(mode ClusterMode, endpointName, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil // empty means "not configured" — handled elsewhere
	}
	if mode == ModeSingleNode {
		return nil // loopback is fine in dev mode
	}
	if IsLoopbackEndpoint(endpoint) {
		return fmt.Errorf(
			"cluster mode: %s=%q uses loopback address — "+
				"cross-service endpoints must be routable between nodes "+
				"(use a DNS name or node IP instead)",
			endpointName, endpoint)
	}
	return nil
}

// ValidateServiceEndpoints checks multiple cross-service endpoints for
// cluster-mode validity. Returns the first error found, or nil if all pass.
func ValidateServiceEndpoints(mode ClusterMode, endpoints map[string]string) error {
	for name, endpoint := range endpoints {
		if err := ValidateServiceEndpoint(mode, name, endpoint); err != nil {
			return err
		}
	}
	return nil
}
