package main

import (
	"fmt"
	"log"
	"net"

	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"google.golang.org/grpc"
)

func main() {
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	address := fmt.Sprintf(":%s", port)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("unable to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()
	srv := NewNodeAgentServer()
	nodeagentpb.RegisterNodeAgentServiceServer(grpcServer, srv)

	log.Printf("node agent listening on %s", address)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}
