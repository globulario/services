package main

import (
	"context"
	"log"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// probeInfraHealth calls a named probe workflow on the node agent at the given
// endpoint via gRPC. The controller CANNOT use os/exec (security constraint),
// so all probes are delegated to node agents.
//
// Returns true if the probe reports Status == "SUCCEEDED".
func (srv *server) probeInfraHealth(ctx context.Context, endpoint, probeName string) bool {
	if endpoint == "" {
		return false
	}

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := srv.dialNodeAgent(endpoint)
	if err != nil {
		log.Printf("infra-probe: failed to connect to %s for probe %s: %v", endpoint, probeName, err)
		return false
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.RunWorkflow(probeCtx, &node_agentpb.RunWorkflowRequest{
		WorkflowName: probeName,
	})
	if err != nil {
		log.Printf("infra-probe: %s on %s failed: %v", probeName, endpoint, err)
		return false
	}

	healthy := resp.GetStatus() == "SUCCEEDED"
	if !healthy {
		log.Printf("infra-probe: %s on %s returned status=%s error=%s",
			probeName, endpoint, resp.GetStatus(), resp.GetError())
	}
	return healthy
}

// probeScyllaHealth probes ScyllaDB health on the given node agent endpoint.
func (srv *server) probeScyllaHealth(ctx context.Context, endpoint string) bool {
	return srv.probeInfraHealth(ctx, endpoint, "probe-scylla-health")
}

// probeEtcdHealth probes etcd health on the given node agent endpoint.
func (srv *server) probeEtcdHealth(ctx context.Context, endpoint string) bool {
	return srv.probeInfraHealth(ctx, endpoint, "probe-etcd-health")
}

// probeMinioHealth probes MinIO health on the given node agent endpoint.
func (srv *server) probeMinioHealth(ctx context.Context, endpoint string) bool {
	return srv.probeInfraHealth(ctx, endpoint, "probe-minio-health")
}

// isActiveInfraMember returns true if the node is an active member of the
// infrastructure cluster identified by pkgName. Active members must NOT be
// reinstalled or disrupted by the release pipeline — doing so would cause
// data loss or cluster instability.
func isActiveInfraMember(node *nodeState, pkgName string) bool {
	if node == nil {
		return false
	}

	name := strings.ToLower(pkgName)

	switch {
	case name == "scylladb" || strings.Contains(name, "scylla"):
		switch node.ScyllaJoinPhase {
		case ScyllaJoinVerified, ScyllaJoinStarted, ScyllaJoinConfigured:
			return true
		}

	case name == "etcd":
		switch node.EtcdJoinPhase {
		case EtcdJoinVerified, EtcdJoinStarted:
			return true
		}

	case name == "minio":
		switch node.MinioJoinPhase {
		case MinioJoinVerified, MinioJoinStarted:
			return true
		}
	}

	return false
}
