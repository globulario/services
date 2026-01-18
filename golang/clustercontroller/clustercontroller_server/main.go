package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/globulario/services/golang/clustercontroller/clustercontroller_server/internal/recovery"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/config"
	planstore "github.com/globulario/services/golang/plan/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	cfgPath := flag.String("config", "/var/lib/globular/cluster-controller/config.json", "cluster controller configuration file")
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

	var planStore planstore.PlanStore
	if etcdClient, err := config.GetEtcdClient(); err == nil {
		planStore = planstore.NewEtcdPlanStore(etcdClient)
	} else {
		log.Printf("plan store unavailable: %v", err)
	}

	address := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", address, err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.Unary(),
		),
		grpc.ChainStreamInterceptor(
			recovery.Stream(),
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	srv := newServer(cfg, *cfgPath, *statePath, state, planStore)
	clustercontrollerpb.RegisterClusterControllerServiceServer(grpcServer, srv)

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		if err := http.ListenAndServe("127.0.0.1:6060", mux); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	srv.startReconcileLoop(context.Background(), 15*time.Second)
	srv.startAgentCleanupLoop(context.Background())
	srv.startOperationCleanupLoop(context.Background())
	srv.startHealthMonitorLoop(context.Background())

	log.Printf("cluster controller listening on %s (config=%s)", address, *cfgPath)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}
