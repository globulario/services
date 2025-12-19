package main

import (
	"context"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type agentClient struct {
	endpoint string
	conn     *grpc.ClientConn
	client   nodeagentpb.NodeAgentServiceClient
}

func newAgentClient(ctx context.Context, endpoint string) (*agentClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, endpoint,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &agentClient{
		endpoint: endpoint,
		conn:     conn,
		client:   nodeagentpb.NewNodeAgentServiceClient(conn),
	}, nil
}

func (a *agentClient) ApplyPlan(ctx context.Context, plan *clustercontrollerpb.NodePlan) error {
	if plan == nil {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_, err := a.client.ApplyPlan(reqCtx, &nodeagentpb.ApplyPlanRequest{Plan: plan})
	return err
}

func (a *agentClient) Close() error {
	if a.conn == nil {
		return nil
	}
	return a.conn.Close()
}
