package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/render"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// ClusterDoctorServer implements ClusterDoctorServiceServer.
type ClusterDoctorServer struct {
	cluster_doctorpb.UnimplementedClusterDoctorServiceServer

	mu           sync.Mutex
	cfg          *clusterdoctorConfig
	collector    *collector.Collector
	registry     *rules.Registry
	version      string

	// cached findings from the last snapshot, keyed by finding_id
	// used by ExplainFinding to avoid re-fetching.
	lastFindings []rules.Finding
	lastFindingsMu sync.RWMutex
}

// buildClientTLSCreds loads the cluster CA and returns gRPC transport credentials
// for outgoing client connections. Falls back to system roots if CA is unavailable.
func buildClientTLSCreds() credentials.TransportCredentials {
	caFile := config.GetTLSFile("", "", "ca.crt")
	if caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				return credentials.NewTLS(&tls.Config{RootCAs: pool})
			}
		}
	}
	// Fallback: system CA pool (still TLS, just not pinned to cluster CA).
	return credentials.NewTLS(&tls.Config{})
}

func newServer(cfg *clusterdoctorConfig, version string) (*ClusterDoctorServer, error) {
	// Dial ClusterController with TLS.
	creds := buildClientTLSCreds()
	ccConn, err := grpc.NewClient(
		cfg.ControllerEndpoint,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("dial clustercontroller %s: %w", cfg.ControllerEndpoint, err)
	}

	ccClient := cluster_controllerpb.NewClusterControllerServiceClient(ccConn)

	col := collector.New(collector.CollectorConfig{
		ListTimeout: cfg.listTimeout(),
		NodeTimeout: cfg.nodeTimeout(),
		Concurrency: cfg.UpstreamNodeConcurrency,
		SnapshotTTL: cfg.snapshotTTL(),
	}, ccClient)

	reg := rules.NewRegistry(rules.Config{
		HeartbeatStale:  cfg.heartbeatStale(),
		EmitAuditEvents: cfg.EmitAuditEvents,
	})

	return &ClusterDoctorServer{
		cfg:       cfg,
		collector: col,
		registry:  reg,
		version:   version,
	}, nil
}

// ─── RPC Handlers ─────────────────────────────────────────────────────────────

func (s *ClusterDoctorServer) GetClusterReport(ctx context.Context, _ *cluster_doctorpb.ClusterReportRequest) (*cluster_doctorpb.ClusterReport, error) {
	snap, err := s.collector.GetSnapshot(ctx)
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	findings := s.registry.EvaluateAll(snap)
	s.cacheFindings(findings)

	return render.ClusterReport(snap, findings, s.version), nil
}

func (s *ClusterDoctorServer) GetNodeReport(ctx context.Context, req *cluster_doctorpb.NodeReportRequest) (*cluster_doctorpb.NodeReport, error) {
	if req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	snap, err := s.collector.GetSnapshot(ctx)
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	findings := s.registry.EvaluateForNode(snap, req.GetNodeId())
	s.cacheFindings(findings)

	return render.NodeReport(snap, req.GetNodeId(), findings, s.version), nil
}

func (s *ClusterDoctorServer) GetDriftReport(ctx context.Context, req *cluster_doctorpb.DriftReportRequest) (*cluster_doctorpb.DriftReport, error) {
	snap, err := s.collector.GetSnapshot(ctx)
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	return render.DriftReport(snap, req.GetNodeId(), s.version), nil
}

func (s *ClusterDoctorServer) ExplainFinding(_ context.Context, req *cluster_doctorpb.ExplainFindingRequest) (*cluster_doctorpb.FindingExplanation, error) {
	if req.GetFindingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "finding_id is required")
	}

	s.lastFindingsMu.RLock()
	cached := make([]rules.Finding, len(s.lastFindings))
	copy(cached, s.lastFindings)
	s.lastFindingsMu.RUnlock()

	f, ok := rules.FindByID(cached, req.GetFindingId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "finding %s not found in last snapshot; call GetClusterReport first", req.GetFindingId())
	}

	return &cluster_doctorpb.FindingExplanation{
		FindingId:   f.FindingID,
		InvariantId: f.InvariantID,
		WhyFailed:   f.Summary,
		Remediation: f.Remediation,
		Evidence:    f.Evidence,
	}, nil
}

// cacheFindings stores the latest findings for ExplainFinding lookups.
func (s *ClusterDoctorServer) cacheFindings(findings []rules.Finding) {
	s.lastFindingsMu.Lock()
	defer s.lastFindingsMu.Unlock()
	merged := make(map[string]rules.Finding, len(s.lastFindings)+len(findings))
	for _, f := range s.lastFindings {
		merged[f.FindingID] = f
	}
	for _, f := range findings {
		merged[f.FindingID] = f
	}
	s.lastFindings = make([]rules.Finding, 0, len(merged))
	for _, f := range merged {
		s.lastFindings = append(s.lastFindings, f)
	}
}
