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
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
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

	grpcServer := grpc.NewServer(
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
	)

	cluster_doctorpb.RegisterClusterDoctorServiceServer(grpcServer, srv)
	logger.Debug("gRPC service registered")

	// Register in Globular service registry so the xDS watcher creates an Envoy cluster.
	// This makes the service reachable via serviceSubdomainUrl('clusterdoctor.ClusterDoctorService')
	// in the TypeScript frontend.
	if err := config.SaveServiceConfiguration(map[string]interface{}{
		"Id":       "cluster_doctor.ClusterDoctorService",
		"Name":     "cluster_doctor.ClusterDoctorService",
		"Address":  "localhost",
		"Port":     cfg.Port,
		"Protocol": "grpc",
		"TLS":      false,
		"State":    "running",
		"Process":  os.Getpid(),
		"Version":  Version,
	}); err != nil {
		logger.Warn("failed to register in Globular service registry; xDS routing may be unavailable", "err", err)
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
	addr := fmt.Sprintf(":%d", pprofPort)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Debug("pprof server stopped", "error", err)
	}
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
