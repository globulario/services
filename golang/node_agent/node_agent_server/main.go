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
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http"
	"path/filepath"

	"github.com/globulario/services/golang/config"
	globular_service "github.com/globulario/services/golang/globular_service"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	address := fmt.Sprintf(":%s", port)

	bootstrapPlanFlag := flag.String("bootstrap-plan", os.Getenv("NODE_AGENT_BOOTSTRAP_PLAN"), "path to bootstrap plan JSON")
	etcdModeFlag := flag.String("etcd-mode", getEnv("NODE_AGENT_ETCD_MODE", "managed"), "etcd mode: managed|external")
	flag.Parse()

	statePath := getEnv("NODE_AGENT_STATE_PATH", "/var/lib/globular/nodeagent/state.json")
	state, err := loadNodeAgentState(statePath)
	if err != nil {
		log.Printf("unable to load node agent state %s: %v", statePath, err)
	}
	srv := NewNodeAgentServer(statePath, state)
	srv.SetEtcdMode(*etcdModeFlag)
	// Plan store removed — workflows handle all execution.
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		var etcdErr error
		if srv.isEtcdManaged() {
			etcdCtx, etcdCancel := context.WithTimeout(ctx, 90*time.Second)
			defer etcdCancel()
			etcdErr = srv.EnsureEtcd(etcdCtx)
			if etcdErr != nil {
				log.Printf("etcd bootstrap failed: %v", etcdErr)
				return
			}
		}
		if err := srv.BootstrapIfNeeded(ctx); err != nil {
			log.Printf("bootstrap plan failed: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("unable to listen on %s: %v", address, err)
	}

	// TLS is mandatory. Check env vars first, then fall back to standard Globular cert paths.
	serverOpts := []grpc.ServerOption{}
	certFile := os.Getenv("NODE_AGENT_TLS_CERT")
	keyFile := os.Getenv("NODE_AGENT_TLS_KEY")
	caFile := os.Getenv("NODE_AGENT_TLS_CA")
	// Use canonical server certs (same paths as framework services).
	// GetTLSFile falls back to envoy-xds-client certs which are CLIENT-only
	// and cause "unsuitable certificate purpose" errors when used as server certs.
	if certFile == "" {
		certFile = config.GetLocalServerCertificatePath()
	}
	if keyFile == "" {
		keyFile = config.GetLocalServerKeyPath()
	}
	if caFile == "" {
		caFile = config.GetLocalCACertificate()
	}
	if certFile != "" && keyFile != "" && caFile != "" {
		tlsCfg := globular_service.GetTLSConfig(keyFile, certFile, caFile)
		if tlsCfg != nil {
			serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
			log.Printf("TLS enabled: cert=%s key=%s ca=%s", certFile, keyFile, caFile)
		} else {
			log.Fatalf("TLS config could not be created — refusing to start insecure")
		}
	} else {
		log.Fatalf("TLS certificate files not found (cert=%s key=%s ca=%s) — refusing to start insecure", certFile, keyFile, caFile)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	// Connect to WorkflowService for plan execution tracing.
	// Uses lazy connection via discoverServiceAddr so the recorder works
	// even when the workflow service isn't available at startup (Day-1 join).
	// The address is re-resolved on each connection attempt — local port
	// first, then gateway fallback.
	wfClusterID := strings.TrimSpace(os.Getenv("CLUSTER_ID"))
	if wfClusterID == "" {
		wfClusterID = "globular.internal"
	}
	wfResolver := func() string {
		if env := strings.TrimSpace(os.Getenv("WORKFLOW_SERVICE_ADDR")); env != "" {
			return env
		}
		return discoverServiceAddr(10220)
	}
	srv.workflowRec = workflow.NewRecorderWithResolver(wfResolver, wfClusterID)
	srv.clusterID = wfClusterID

	srv.StartHeartbeat(ctx)
	// Plan runner removed — workflows handle all execution.
	srv.StartACMERenewal(ctx)
	srv.StartCAKeySync(ctx)
	srv.StartIngressReconciliation(ctx)
	node_agentpb.RegisterNodeAgentServiceServer(grpcServer, srv)

	// Register in the Globular service registry so the xDS watcher creates an Envoy cluster.
	// NodeAgent is a standalone control-plane service that does not use the
	// globular_service framework; without this call it is invisible to service discovery.
	if portNum, convErr := strconv.Atoi(port); convErr == nil {
		// Use the real advertised IP so that remote cluster-controller instances
		// can discover and reach this node agent.  advertisedAddr is "ip:port";
		// we only need the host part for the registry "Address" field.
		advertiseHost := strings.Split(srv.advertisedAddr, ":")[0]
		if regErr := config.SaveServiceConfiguration(map[string]interface{}{
			"Id":       "node_agent.NodeAgentService",
			"Name":     "node_agent.NodeAgentService",
			"Address":  advertiseHost,
			"Port":     portNum,
			"Protocol": "grpc",
			"TLS":      true,
			"State":    "running",
			"Process":  os.Getpid(),
			"Version":  getEnv("NODE_AGENT_VERSION", "0.0.1"),
		}); regErr != nil {
			log.Printf("warn: failed to register in Globular service registry; xDS routing may be unavailable: %v", regErr)
		}
	}

	// Start Prometheus metrics server on a free port.
	go startMetricsServer()

	log.Printf("node agent listening on %s", address)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}
}

func startMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("metrics: listen failed: %v", err)
		return
	}
	metricsPort := ln.Addr().(*net.TCPAddr).Port
	log.Printf("metrics listening on 127.0.0.1:%d", metricsPort)
	writePromTargetFile("node_agent", metricsPort)
	if err := http.Serve(ln, mux); err != nil {
		log.Printf("metrics server error: %v", err)
	}
}

const promTargetsDir = "/var/lib/globular/prometheus/targets"

func writePromTargetFile(job string, port int) {
	content := fmt.Sprintf("- targets: [\"127.0.0.1:%d\"]\n  labels:\n    job: %s\n", port, job)
	if err := os.MkdirAll(promTargetsDir, 0750); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(promTargetsDir, job+".yaml"), []byte(content), 0644)
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
