package main

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
)

const doctorServiceName = "cluster_doctor.ClusterDoctorService"

// resolveDoctorEndpoint enforces the control-plane dial invariant:
// CLI gRPC calls must target TLS-routable endpoints. Service discovery
// already rewrites to mesh host:443 for external callers.
func resolveDoctorEndpoint(override string) (string, error) {
	endpoint := strings.TrimSpace(override)
	if endpoint == "" {
		endpoint = strings.TrimSpace(config.ResolveServiceAddr(doctorServiceName, ""))
	}
	if endpoint == "" {
		return "", fmt.Errorf("cluster-doctor endpoint not found (use --endpoint or check service registration)")
	}
	resolved, err := resolveGRPCAddr(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid doctor endpoint %q: %w", endpoint, err)
	}
	return resolved, nil
}
