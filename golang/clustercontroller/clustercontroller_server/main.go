package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
)

func main() {
	cfgPath := flag.String("config", "/etc/globular/clustercontroller/config.json", "cluster controller configuration file")
	flag.Parse()

	cfg, err := loadClusterControllerConfig(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load config %s: %v", *cfgPath, err)
	}

	address := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()
	srv := newServer(cfg, *cfgPath)
	clustercontrollerpb.RegisterClusterControllerServiceServer(grpcServer, srv)

	log.Printf("cluster controller listening on %s (config=%s)", address, *cfgPath)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}
