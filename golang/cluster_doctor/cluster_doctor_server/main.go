package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/internal/recovery"
	"github.com/globulario/services/golang/config"
	globular_service "github.com/globulario/services/golang/globular_service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"path/filepath"
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

const (
	defaultConfigPath = "/var/lib/globular/clusterdoctor/config.json"
	pprofPort         = 6070
)

func main() {
	cfgPath := flag.String("config", defaultConfigPath, "path to clusterdoctor config file")
	describe := flag.Bool("describe", false, "print service metadata as JSON and exit")
	health := flag.Bool("health", false, "print service health as JSON and exit")
	version := flag.Bool("version", false, "print version information and exit")
	debug := flag.Bool("debug", false, "enable debug logging")
	help := flag.Bool("help", false, "show usage information")
	flag.Parse()

	if *debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *version {
		printVersion()
		os.Exit(0)
	}
	if *describe {
		printDescribe()
		os.Exit(0)
	}
	if *health {
		printHealth()
		os.Exit(0)
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		logger.Error("failed to load config", "path", *cfgPath, "error", err)
		os.Exit(1)
	}
	if err := cfg.validate(); err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	srv, err := newServer(cfg, Version)
	if err != nil {
		logger.Error("failed to create server", "error", err)
		os.Exit(1)
	}
	defer srv.collector.Close()

	address := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("failed to listen", "address", address, "error", err)
		os.Exit(1)
	}

	// TLS is mandatory — use the same certificate paths as all other Globular services.
	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(recovery.Unary()),
		grpc.ChainStreamInterceptor(recovery.Stream()),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	// Use canonical server certs (same paths as framework services).
	// GetTLSFile falls back to envoy-xds-client certs which are CLIENT-only
	// and cause "unsuitable certificate purpose" errors when used as server certs.
	certFile := config.GetLocalServerCertificatePath()
	keyFile := config.GetLocalServerKeyPath()
	caFile := config.GetLocalCACertificate()
	if certFile != "" && keyFile != "" && caFile != "" {
		tlsCfg := globular_service.GetTLSConfig(keyFile, certFile, caFile)
		if tlsCfg != nil {
			serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
			logger.Info("TLS enabled", "cert", certFile, "key", keyFile, "ca", caFile)
		} else {
			logger.Error("TLS config could not be created — refusing to start insecure")
			os.Exit(1)
		}
	} else {
		logger.Error("TLS certificate files not found — refusing to start insecure",
			"cert", certFile, "key", keyFile, "ca", caFile)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(serverOpts...)

	cluster_doctorpb.RegisterClusterDoctorServiceServer(grpcServer, srv)
	logger.Debug("gRPC service registered")

	// Register in Globular service registry so the xDS watcher creates an Envoy cluster.
	// This makes the service reachable via serviceSubdomainUrl('clusterdoctor.ClusterDoctorService')
	// in the TypeScript frontend.
	// Retry in background if etcd isn't ready yet (boot-order race).
	regCfg := map[string]interface{}{
		"Id":       "cluster_doctor.ClusterDoctorService",
		"Name":     "cluster_doctor.ClusterDoctorService",
		"Address":  config.GetRoutableIPv4(),
		"Port":     cfg.Port,
		"Protocol": "grpc",
		"TLS":      true,
		"State":    "running",
		"Process":  os.Getpid(),
		"Version":  Version,
	}
	if err := config.SaveServiceConfiguration(regCfg); err != nil {
		logger.Warn("service registry registration failed, retrying in background", "err", err)
		go func() {
			for i := 0; i < 12; i++ {
				time.Sleep(10 * time.Second)
				if err := config.SaveServiceConfiguration(regCfg); err == nil {
					logger.Info("service registry registration succeeded (background retry)")
					return
				}
			}
			logger.Error("service registry registration failed after all retries; xDS routing unavailable")
		}()
	}

	go startPprofServer()

	logger.Info("cluster doctor ready",
		"address", address,
		"controller", cfg.ControllerEndpoint,
		"version", Version,
	)

	// Graceful shutdown on SIGTERM / SIGINT
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		logger.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = ctx
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("grpc serve failed", "error", err)
		os.Exit(1)
	}
}

func startPprofServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/metrics", promhttp.Handler())

	// Bind to :0 so we get a free port, then register with Prometheus.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("metrics/pprof: listen failed", "error", err)
		return
	}
	port := ln.Addr().(*net.TCPAddr).Port
	logger.Info("metrics/pprof listening", "addr", ln.Addr().String())
	writePromTargetFile("cluster_doctor", port)

	if err := http.Serve(ln, mux); err != nil {
		logger.Error("metrics/pprof server error", "error", err)
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

func printVersion() {
	info := map[string]string{
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

func printDescribe() {
	metadata := map[string]interface{}{
		"name":        "cluster-doctor",
		"version":     Version,
		"description": "Globular cluster doctor — deterministic, read-only operational intelligence for a Globular cluster",
		"grpc_port":   12100,
		"pprof_port":  pprofPort,
		"capabilities": []string{
			"health-analysis",
			"drift-detection",
			"plan-explanation",
			"remediation-proposals",
		},
		"build_info": map[string]string{
			"version":    Version,
			"build_time": BuildTime,
			"git_commit": GitCommit,
		},
	}
	data, _ := json.MarshalIndent(metadata, "", "  ")
	fmt.Println(string(data))
}

func printHealth() {
	health := map[string]interface{}{
		"status":  "unknown",
		"message": "health check requires running server instance",
	}
	data, _ := json.MarshalIndent(health, "", "  ")
	fmt.Println(string(data))
}
