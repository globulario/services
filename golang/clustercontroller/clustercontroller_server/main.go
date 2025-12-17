package main

import (
	"fmt"
	"log"
	"net"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
)

func main() {
	port := 12000
	address := fmt.Sprintf(":%d", port)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer()
	srv := &server{}
	clustercontrollerpb.RegisterClusterControllerServiceServer(grpcServer, srv)

	log.Printf("cluster controller listening on %s", address)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}
