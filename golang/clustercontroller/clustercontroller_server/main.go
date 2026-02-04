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
	"strings"
	"time"

	"github.com/globulario/services/golang/clustercontroller/clustercontroller_server/internal/recovery"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/config"
	planstore "github.com/globulario/services/golang/plan/store"
	clientv3 "go.etcd.io/etcd/client/v3"
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

	var (
		planStore  planstore.PlanStore
		etcdClient *clientv3.Client
	)
	if c, err := config.GetEtcdClient(); err == nil {
		etcdClient = c
		planStore = planstore.NewEtcdPlanStore(c)
	} else {
		log.Printf("plan store unavailable: %v", err)
	}
	if etcdClient != nil {
		defer etcdClient.Close()
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
	srv.initResourceStore(etcdClient)
	clustercontrollerpb.RegisterClusterControllerServiceServer(grpcServer, srv)
	clustercontrollerpb.RegisterResourcesServiceServer(grpcServer, srv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	leaderAddr := resolveLeaderAddr(address)
	bootstrapLeadership(ctx, srv, etcdClient, leaderAddr)

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

	srv.startControllerRuntime(ctx, 4)
	srv.startAgentCleanupLoop(context.Background())
	srv.startOperationCleanupLoop(context.Background())
	srv.startHealthMonitorLoop(context.Background())

	// Start DNS reconciler (PR2) - only if cluster_domain configured
	// PR7: Support multiple DNS endpoints for high availability
	if cfg.ClusterDomain != "" {
		dnsEndpointsStr := os.Getenv("CLUSTER_DNS_ENDPOINTS")
		if dnsEndpointsStr == "" {
			dnsEndpointsStr = "127.0.0.1:10033"
		}

		// Parse comma-separated list of DNS endpoints
		dnsEndpoints := strings.Split(dnsEndpointsStr, ",")
		for i := range dnsEndpoints {
			dnsEndpoints[i] = strings.TrimSpace(dnsEndpoints[i])
		}

		dnsReconciler := NewDNSReconciler(srv, dnsEndpoints)
		dnsReconciler.Start()
		log.Printf("dns reconciler: ENABLED (domain=%s, endpoints=%v)", cfg.ClusterDomain, dnsEndpoints)
	} else {
		log.Printf("dns reconciler: DISABLED (no cluster_domain configured)")
	}

	log.Printf("cluster controller listening on %s (config=%s)", address, *cfgPath)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}
