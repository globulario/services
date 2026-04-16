package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"crypto/tls"
	"crypto/x509"
	"os"
)

// triggerJoinWorkflow calls NodeAgent.RunWorkflow on the joining node
// to execute the node.join workflow definition. This replaces the
// old reconcile loop with a single gRPC call.
//
// Callers MUST set BootstrapWorkflowActive = true under the state lock
// before launching this goroutine to prevent double-triggers.
func (srv *server) triggerJoinWorkflow(nodeID, agentEndpoint string) {
	// Clear BootstrapWorkflowActive when we exit, regardless of outcome,
	// so the recovery mechanism can re-trigger if needed.
	defer func() {
		srv.lock("triggerJoinWorkflow:deactivate")
		if n := srv.state.Nodes[nodeID]; n != nil {
			n.BootstrapWorkflowActive = false
		}
		srv.unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Printf("workflow-trigger: connecting to node-agent at %s for node %s", agentEndpoint, nodeID)

	conn, err := srv.dialNodeAgent(agentEndpoint)
	if err != nil {
		log.Printf("workflow-trigger: failed to connect to %s: %v", agentEndpoint, err)
		return
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)

	resp, err := client.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
		WorkflowName: "node.join",
	})
	if err != nil {
		log.Printf("workflow-trigger: RunWorkflow failed for node %s: %v", nodeID, err)
		return
	}

	log.Printf("workflow-trigger: node %s workflow completed — status=%s steps=%d/%d duration=%dms",
		nodeID, resp.GetStatus(), resp.GetStepsSucceeded(), resp.GetStepsTotal(), resp.GetDurationMs())

	if resp.GetError() != "" {
		log.Printf("workflow-trigger: node %s error: %s", nodeID, resp.GetError())
	}

	// Chain: after join workflow completes successfully, run the bootstrap
	// workflow locally on the controller to advance the node through
	// admitted → workload_ready.
	if resp.GetStatus() == "SUCCEEDED" {
		srv.triggerBootstrapWorkflow(nodeID)
	}
}

// triggerBootstrapWorkflow runs the node.bootstrap workflow locally on the
// controller. It advances the node from admitted → workload_ready by
// polling in-memory node state for convergence conditions.
func (srv *server) triggerBootstrapWorkflow(nodeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Printf("workflow-trigger: starting bootstrap workflow for node %s", nodeID)
	resp, err := srv.RunBootstrapWorkflow(ctx, nodeID)
	if err != nil {
		log.Printf("workflow-trigger: bootstrap workflow failed for node %s: %v", nodeID, err)
		return
	}
	log.Printf("workflow-trigger: bootstrap workflow for node %s completed — status=%s", nodeID, resp.Status)
}

// dialNodeAgent creates a direct gRPC connection to a node-agent.
// Uses config.ResolveDialTarget for canonical endpoint resolution.
func (srv *server) dialNodeAgent(endpoint string) (*grpc.ClientConn, error) {
	if srv.testDialNodeAgent != nil {
		return srv.testDialNodeAgent(endpoint)
	}
	dt := config.ResolveDialTarget(endpoint)

	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	caFile := "/var/lib/globular/pki/ca.crt"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   dt.ServerName,
	})

	// Add node token as metadata for auth.
	token, _ := security.GetLocalToken("")

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenAuth{token: token}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return grpc.DialContext(ctx, dt.Address, opts...)
}

// tokenAuth implements grpc.PerRPCCredentials for bearer token auth.
type tokenAuth struct {
	token string
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"token": t.token}, nil
}

func (t tokenAuth) RequireTransportSecurity() bool { return false }
