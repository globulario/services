package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/render"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)


// ClusterDoctorServer implements ClusterDoctorServiceServer.
type ClusterDoctorServer struct {
	cluster_doctorpb.UnimplementedClusterDoctorServiceServer

	// isAuthoritative is true when this instance is the elected leader.
	// Only the leader produces fresh findings. Followers serve cached data
	// with source="follower" in freshness headers.
	isAuthoritative atomic.Bool

	mu           sync.Mutex
	cfg          *clusterdoctorConfig
	collector    *collector.Collector
	registry     *rules.Registry
	version      string
	eventClient  *event_client.Event_Client

	// cached findings from the last snapshot, keyed by finding_id
	// used by ExplainFinding to avoid re-fetching.
	lastFindings []rules.Finding
	lastFindingsMu sync.RWMutex

	// executor runs structured RemediationActions with hardcoded blocklists.
	// Optional: nil means ExecuteRemediation returns a not-configured error.
	executor *ActionExecutor

	// workflowClient is used to delegate workflow execution to the
	// centralized WorkflowService. Set during newServer() if
	// WorkflowEndpoint is configured.
	workflowClient workflowpb.WorkflowServiceClient
	clusterID      string
}

// buildClientTLSCreds loads the cluster CA and returns gRPC transport
// credentials for outgoing client connections, with ServerName pinned to
// the cert-valid hostname chosen by config.ResolveDialTarget. Falls back
// to system roots if CA is unavailable.
//
// The serverName argument must be the DialTarget.ServerName (never an
// IP literal) — it is what TLS verifies the peer certificate against.
//
// Loopback rewrite: config.ResolveDialTarget rewrites 127.0.0.1/::1 to
// "localhost", but service certs in the cluster never include "localhost"
// in their SAN list (they use the real hostname + *.cluster-domain).
// When we detect a localhost ServerName, substitute the machine hostname
// so the TLS handshake verifies against a SAN that actually exists in
// the cert. This unblocks doctor's ListNodes and fetchPerNode which
// would otherwise fail with "certificate is valid for globule-X, not localhost".
//
// Additionally, we also load a client certificate (mTLS) so services
// that require it (e.g. node_agent's VerifyPackageIntegrity with
// permission=read) see an authenticated peer identity, not an
// anonymous TLS-only connection.
func buildClientTLSCreds(serverName string) credentials.TransportCredentials {
	if serverName == "" || serverName == "localhost" || serverName == "::1" {
		if h, err := os.Hostname(); err == nil && h != "" {
			serverName = h
		}
	}
	tlsCfg := &tls.Config{ServerName: serverName}
	caFile := config.GetTLSFile("", "", "ca.crt")
	if caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				tlsCfg.RootCAs = pool
			}
		}
	}
	// Best-effort mTLS client cert — required by some RPCs for auth.
	clientCert := "/var/lib/globular/pki/issued/services/service.crt"
	clientKey := "/var/lib/globular/pki/issued/services/service.key"
	if cert, err := tls.LoadX509KeyPair(clientCert, clientKey); err == nil {
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsCfg)
}

func newServer(cfg *clusterdoctorConfig, version string) (*ClusterDoctorServer, error) {
	// Dial ClusterController with TLS. Endpoint resolution (loopback
	// rewrite + SNI) happens once, here — not scattered across helpers.
	ccTarget := config.ResolveDialTarget(cfg.ControllerEndpoint)
	ccConn, err := grpc.NewClient(
		ccTarget.Address,
		grpc.WithTransportCredentials(buildClientTLSCreds(ccTarget.ServerName)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial clustercontroller %s: %w", ccTarget.Address, err)
	}

	ccClient := cluster_controllerpb.NewClusterControllerServiceClient(ccConn)

	col := collector.New(collector.CollectorConfig{
		ListTimeout: cfg.listTimeout(),
		NodeTimeout: cfg.nodeTimeout(),
		Concurrency: cfg.UpstreamNodeConcurrency,
		SnapshotTTL: cfg.snapshotTTL(),
	}, ccClient)

	// Attach workflow-service client for convergence telemetry and
	// centralized workflow execution (optional).
	//
	// Resolve endpoint dynamically from etcd service registry first
	// (source of truth for address + port), falling back to the config
	// default only if etcd is unreachable. This avoids hardcoding a port
	// that may not match the actual running workflow service.
	var wfClient workflowpb.WorkflowServiceClient
	clusterID := cfg.ClusterID
	wfEndpoint := config.ResolveServiceAddr("workflow.WorkflowService", cfg.WorkflowEndpoint)
	if wfEndpoint == "" {
		wfEndpoint = cfg.WorkflowEndpoint // last-resort compiled default
	}
	if wfEndpoint != "" {
		wfTarget := config.ResolveDialTarget(wfEndpoint)
		wfConn, wfErr := grpc.NewClient(wfTarget.Address, grpc.WithTransportCredentials(buildClientTLSCreds(wfTarget.ServerName)))
		if wfErr == nil {
			if clusterID == "" {
				// Auto-discover cluster_id from the controller so operators
				// don't have to duplicate it in the doctor config.
				infoCtx, cancel := context.WithTimeout(context.Background(), cfg.listTimeout())
				if info, err := ccClient.GetClusterInfo(infoCtx, nil); err == nil && info != nil {
					clusterID = info.GetClusterId()
				}
				cancel()
			}
			wfClient = workflowpb.NewWorkflowServiceClient(wfConn)
			col.WithWorkflowClient(wfClient, clusterID)
		}
	}

	reg := rules.NewRegistry(rules.Config{
		HeartbeatStale:  cfg.heartbeatStale(),
		EmitAuditEvents: cfg.EmitAuditEvents,
	})

	s := &ClusterDoctorServer{
		cfg:            cfg,
		collector:      col,
		registry:       reg,
		version:        version,
		executor:       &ActionExecutor{nodeAgentDialer: newControllerNodeAgentDialer(ccClient)},
		workflowClient: wfClient,
		clusterID:      clusterID,
	}

	// Event client for publishing finding deltas to ai-watcher (optional).
	if cfg.EmitAuditEvents {
		// Dial the local event service via its in-cluster address.
		// Default to localhost (not 127.0.0.1) so the TLS cert's
		// Resolve event service from etcd (source of truth).
		addr := config.ResolveServiceAddr("event.EventService", "")
		if addr != "" {
			if ec, err := event_client.NewEventService_Client(addr, "event.EventService"); err == nil {
				s.eventClient = ec
			} else {
				logger.Warn("event client init failed (finding events disabled)", "err", err)
			}
		}
	}

	return s, nil
}

// ─── RPC Handlers ─────────────────────────────────────────────────────────────

// resolveFreshnessMode normalises a caller's FreshnessMode into the
// effective mode honoured by the server. UNSPECIFIED defaults to
// CACHED (the current behaviour before this contract existed).
func resolveFreshnessMode(req cluster_doctorpb.FreshnessMode) cluster_doctorpb.FreshnessMode {
	if req == cluster_doctorpb.FreshnessMode_FRESHNESS_UNSPECIFIED {
		return cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED
	}
	return req
}

// takeSnapshot wraps the collector fetch so each handler uses identical
// freshness-resolution logic. Returns the snapshot plus the Freshness
// bundle the render layer stamps into ReportHeader.
//
// Followers never force-fresh — they always serve cached data to prevent
// duplicate upstream scans. The freshness header discloses authority status.
func (s *ClusterDoctorServer) takeSnapshot(ctx context.Context, requested cluster_doctorpb.FreshnessMode) (*collector.Snapshot, render.Freshness, error) {
	mode := resolveFreshnessMode(requested)
	// Only the leader may force-fresh. Followers always serve cached.
	forceFresh := mode == cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH && s.isAuthoritative.Load()
	if !s.isAuthoritative.Load() {
		mode = cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED
	}
	res, err := s.collector.GetSnapshotWithFreshness(ctx, forceFresh)
	fresh := render.Freshness{
		CacheHit:  res.CacheHit,
		CacheTTL:  res.CacheTTL,
		Mode:      mode,
		Authority: s.authoritySource(),
	}
	return res.Snapshot, fresh, err
}

func (s *ClusterDoctorServer) GetClusterReport(ctx context.Context, req *cluster_doctorpb.ClusterReportRequest) (*cluster_doctorpb.ClusterReport, error) {
	snap, fresh, err := s.takeSnapshot(ctx, req.GetFreshness())
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	findings := s.registry.EvaluateAll(snap)
	s.cacheFindings(findings)

	return render.ClusterReport(snap, findings, s.version, fresh), nil
}

func (s *ClusterDoctorServer) GetNodeReport(ctx context.Context, req *cluster_doctorpb.NodeReportRequest) (*cluster_doctorpb.NodeReport, error) {
	if req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	snap, fresh, err := s.takeSnapshot(ctx, req.GetFreshness())
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	findings := s.registry.EvaluateForNode(snap, req.GetNodeId())
	s.cacheFindings(findings)

	return render.NodeReport(snap, req.GetNodeId(), findings, s.version, fresh), nil
}

func (s *ClusterDoctorServer) GetDriftReport(ctx context.Context, req *cluster_doctorpb.DriftReportRequest) (*cluster_doctorpb.DriftReport, error) {
	snap, fresh, err := s.takeSnapshot(ctx, req.GetFreshness())
	if err != nil && snap == nil {
		return nil, status.Errorf(codes.Internal, "snapshot fetch failed: %v", err)
	}

	return render.DriftReport(snap, req.GetNodeId(), s.version, fresh), nil
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

// cacheFindings stores the latest findings for ExplainFinding lookups and
// emits cluster.finding.created / cluster.finding.resolved events on each
// snapshot delta so ai-watcher (and any operator) can react to changes.
func (s *ClusterDoctorServer) cacheFindings(findings []rules.Finding) {
	s.lastFindingsMu.Lock()
	// Index current findings by ID.
	current := make(map[string]rules.Finding, len(findings))
	for _, f := range findings {
		current[f.FindingID] = f
	}
	// Compute delta vs previous snapshot.
	prev := make(map[string]rules.Finding, len(s.lastFindings))
	for _, f := range s.lastFindings {
		prev[f.FindingID] = f
	}
	var created, resolved []rules.Finding
	for id, f := range current {
		if _, had := prev[id]; !had {
			created = append(created, f)
		}
	}
	for id, f := range prev {
		if _, still := current[id]; !still {
			resolved = append(resolved, f)
		}
	}
	// Replace cache with the LATEST evaluation only (drop stale entries).
	s.lastFindings = findings
	s.lastFindingsMu.Unlock()

	// Emit events outside the lock.
	if s.cfg.EmitAuditEvents {
		for _, f := range created {
			s.publishFindingEvent("cluster.finding.created", f)
		}
		for _, f := range resolved {
			s.publishFindingEvent("cluster.finding.resolved", f)
		}
	}
}

// publishFindingEvent sends one finding event to the event service. The
// payload is small and queryable — just the data ai-watcher needs to decide
// whether to trigger a diagnosis run.
func (s *ClusterDoctorServer) publishFindingEvent(topic string, f rules.Finding) {
	if s.eventClient == nil {
		return
	}
	payload := map[string]string{
		"finding_id":   f.FindingID,
		"invariant_id": f.InvariantID,
		"severity":     f.Severity.String(),
		"category":     f.Category,
		"entity_ref":   f.EntityRef,
		"summary":      f.Summary,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if err := s.eventClient.Publish(topic, data); err != nil {
		logger.Warn("publish finding event failed", "topic", topic, "err", err)
	}
}
