package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
)

func main() {
	cfgPath := flag.String("config", "/etc/globular/clustercontroller/config.json", "cluster controller configuration file")
	statePath := flag.String("state", defaultClusterStatePath, "cluster controller state file")
	flag.Parse()

	if env := os.Getenv("CLUSTER_STATE_PATH"); env != "" {
		*statePath = env
	}

	cfg, err := loadClusterControllerConfig(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load config %s: %v", *cfgPath, err)
	}

	state, err := loadControllerState(*statePath)
	if err != nil {
		log.Fatalf("failed to load state %s: %v", *statePath, err)
	}

	address := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()
	srv := newServer(cfg, *cfgPath, *statePath, state)
	clustercontrollerpb.RegisterClusterControllerServiceServer(grpcServer, srv)

	srv.startReconcileLoop(context.Background(), 15*time.Second)
	srv.startAgentCleanupLoop(context.Background())

	log.Printf("cluster controller listening on %s (config=%s)", address, *cfgPath)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}
