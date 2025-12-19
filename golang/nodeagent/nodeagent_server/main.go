package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"google.golang.org/grpc"
)

func main() {
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	address := fmt.Sprintf(":%s", port)

	bootstrapPlanFlag := flag.String("bootstrap-plan", os.Getenv("NODE_AGENT_BOOTSTRAP_PLAN"), "path to bootstrap plan JSON")
	flag.Parse()

	statePath := getEnv("NODE_AGENT_STATE_PATH", "/var/lib/globular/nodeagent/state.json")
	state, err := loadNodeAgentState(statePath)
	if err != nil {
		log.Printf("unable to load node agent state %s: %v", statePath, err)
	}
	srv := NewNodeAgentServer(statePath, state)
	if planPath := strings.TrimSpace(*bootstrapPlanFlag); planPath != "" {
		if plan, err := loadBootstrapPlan(planPath); err != nil {
			log.Printf("unable to load bootstrap plan %s: %v", planPath, err)
		} else if len(plan) > 0 {
			srv.SetBootstrapPlan(plan)
			log.Printf("bootstrap plan loaded from %s", planPath)
		}
	}

	if srv.state != nil && srv.state.RequestID != "" && srv.nodeID == "" {
		srv.startJoinApprovalWatcher(context.Background(), srv.state.RequestID)
	}

	go func() {
		if err := srv.BootstrapIfNeeded(context.Background()); err != nil {
			log.Printf("bootstrap plan failed: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("unable to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()
	nodeagentpb.RegisterNodeAgentServiceServer(grpcServer, srv)

	srv.StartHeartbeat(context.Background())

	log.Printf("node agent listening on %s", address)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}

func loadBootstrapPlan(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, nil
	}
	var plan []string
	if err := json.Unmarshal(b, &plan); err != nil {
		return nil, err
	}
	clean := make([]string, 0, len(plan))
	for _, svc := range plan {
		if svc = strings.TrimSpace(svc); svc != "" {
			clean = append(clean, svc)
		}
	}
	return clean, nil
}
