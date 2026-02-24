package main

import (
	"context"
	"fmt"
	"sync"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/render"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/rules"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ClusterDoctorServer implements ClusterDoctorServiceServer.
type ClusterDoctorServer struct {
	clusterdoctorpb.UnimplementedClusterDoctorServiceServer

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

func newServer(cfg *clusterdoctorConfig, version string) (*ClusterDoctorServer, error) {
	// Dial ClusterController
	ccConn, err := grpc.NewClient(
		cfg.ControllerEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial clustercontroller %s: %w", cfg.ControllerEndpoint, err)
	}

	ccClient := clustercontrollerpb.NewClusterControllerServiceClient(ccConn)

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

func (s *ClusterDoctorServer) GetClusterReport(ctx context.Context, _ *clusterdoctorpb.ClusterReportRequest) (*clusterdoctorpb.ClusterReport, error) {
	snap, err := s.collector.GetSnapshot(ctx)
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	findings := s.registry.EvaluateAll(snap)
	s.cacheFindings(findings)

	return render.ClusterReport(snap, findings, s.version), nil
}

func (s *ClusterDoctorServer) GetNodeReport(ctx context.Context, req *clusterdoctorpb.NodeReportRequest) (*clusterdoctorpb.NodeReport, error) {
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

func (s *ClusterDoctorServer) GetDriftReport(ctx context.Context, req *clusterdoctorpb.DriftReportRequest) (*clusterdoctorpb.DriftReport, error) {
	snap, err := s.collector.GetSnapshot(ctx)
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	return render.DriftReport(snap, req.GetNodeId(), s.version), nil
}

func (s *ClusterDoctorServer) ExplainFinding(_ context.Context, req *clusterdoctorpb.ExplainFindingRequest) (*clusterdoctorpb.FindingExplanation, error) {
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

	return &clusterdoctorpb.FindingExplanation{
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
