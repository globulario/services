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
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/render"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/remediation"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ClusterDoctorServer implements ClusterDoctorServiceServer.
type ClusterDoctorServer struct {
	cluster_doctorpb.UnimplementedClusterDoctorServiceServer

	// isAuthoritative is true when this instance is the elected leader.
	// Only the leader produces fresh findings. Followers serve cached data
	// with source="follower" in freshness headers.
	isAuthoritative atomic.Bool

	mu          sync.Mutex
	cfg         *clusterdoctorConfig
	collector   *collector.Collector
	registry    *rules.Registry
	version     string
	eventClient *event_client.Event_Client

	// cached findings from the last snapshot, keyed by finding_id
	// used by ExplainFinding to avoid re-fetching. Any caller (cluster-wide
	// or node-scoped) may populate this — it is a lookup cache, not the
	// authority for change detection.
	lastFindings   []rules.Finding
	lastFindingsMu sync.RWMutex

	// runFindings binds the exact finding an autonomous run was started for to
	// that run's id, so the workflow's resolve callback cannot substitute a
	// different finding by re-reading the mutable lastFindings cache between
	// selection and dispatch. See run_finding_binding.go.
	runFindings runFindingBindings

	// lastEmittedFindings is the most recent CLUSTER-WIDE finding set used to
	// compute the create/resolve delta emitted as cluster.finding.* events.
	// It is intentionally separate from lastFindings so that a node-scoped
	// GetNodeReport (which returns a subset) cannot corrupt the cluster-wide
	// delta and produce spurious "resolved → created" event churn on the
	// next cluster-wide call. Only cluster-wide paths update this.
	lastEmittedFindings []rules.Finding

	// executor runs structured RemediationActions with hardcoded blocklists.
	// Optional: nil means ExecuteRemediation returns a not-configured error.
	executor *ActionExecutor

	// workflowClient is used to delegate workflow execution to the
	// centralized WorkflowService. Set during newServer() if
	// WorkflowEndpoint is configured.
	workflowClient workflowpb.WorkflowServiceClient
	clusterID      string

	// behavioralRecorder delivers doctor findings to behavioral-memory as
	// governed observations. OPTIONAL: nil means behavioral learning is
	// unavailable, which degrades learning only — never report generation.
	// Delivery is bounded and non-blocking; the recorder owns queueing,
	// retry, and worker concurrency so the report path stays synchronous.
	behavioralRecorder behavioralObservationRecorder

	// behavioralGovernor answers whether a remediation may be dispatched, and
	// records what it achieved. SYNCHRONOUS, unlike the recorder: its answer
	// decides whether an action happens. OPTIONAL — nil means no governor is
	// wired and the doctor's own executor gates decide alone.
	behavioralGovernor behavioralGovernor

	// naDialer resolves node_agent endpoints via the cluster controller
	// and dials them with TLS. Used by the ActionExecutor for typed
	// remediation actions (SYSTEMCTL_*, FILE_DELETE on node agents).
	naDialer *controllerNodeAgentDialer

	// auditRing stores recent periodic heal reports for inspection.
	auditRing *healerAuditRing
	// auditStore is the persistent JSONL file for heal action history.
	auditStore *rules.HealAuditStore
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

// clusterIDInjectingUnaryInterceptor injects the local cluster_id into
// outgoing gRPC metadata when not already present. Mirrors the behaviour
// of globular_client.clientInterceptor so that doctor's raw grpc.NewClient
// dials pass the server-side cluster-membership check enforced after Day-0
// initialization (rpc error: "cluster_id required after cluster initialization").
func clusterIDInjectingUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_id")) == 0 {
			if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "cluster_id", clusterID)
			}
		}
		// Also carry the opaque membership UUID (identity), not just the namespace.
		ctx = security.AppendClusterUIDMetadata(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// clusterIDInjectingStreamInterceptor is the streaming counterpart of
// clusterIDInjectingUnaryInterceptor. Applied alongside the unary one so
// future streaming RPCs from the doctor also carry cluster_id metadata.
func clusterIDInjectingStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_id")) == 0 {
			if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "cluster_id", clusterID)
			}
		}
		// Also carry the opaque membership UUID (identity), not just the namespace.
		ctx = security.AppendClusterUIDMetadata(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// dialOptionsForInternalService bundles the dial options used by all of
// the doctor's outgoing clients (controller, workflow, repository,
// ai-memory, awareness-graph). Each dial gets the standard TLS creds and
// the cluster_id-injecting interceptors so server-side membership checks
// don't reject the call.
func dialOptionsForInternalService(serverName string) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(buildClientTLSCreds(serverName)),
		grpc.WithUnaryInterceptor(clusterIDInjectingUnaryInterceptor()),
		grpc.WithStreamInterceptor(clusterIDInjectingStreamInterceptor()),
	}
}

func newServer(cfg *clusterdoctorConfig, version string) (*ClusterDoctorServer, error) {
	// Resolve controller endpoint from etcd (source of truth), falling
	// back to config file value only if etcd is unreachable.
	ccEndpoint := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", cfg.ControllerEndpoint)
	if ccEndpoint == "" {
		ccEndpoint = cfg.ControllerEndpoint
	}
	if ccEndpoint == "" {
		return nil, fmt.Errorf("controller endpoint not configured and not found in etcd")
	}

	ccTarget := config.ResolveDialTarget(ccEndpoint)
	ccConn, err := grpc.NewClient(
		ccTarget.Address,
		dialOptionsForInternalService(ccTarget.ServerName)...,
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
		wfConn, wfErr := grpc.NewClient(wfTarget.Address, dialOptionsForInternalService(wfTarget.ServerName)...)
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

	// Attach repository-service client for ListRepositoryFindings + GetRepositoryStatus
	// + the release-boundary VerifyArtifact probe.
	// Resolved from etcd; optional — if unreachable, repository invariants degrade gracefully.
	//
	// DIRECT dial (not the Envoy mesh :443): auth-gated repo RPCs like
	// VerifyArtifact require the caller's identity. Through the mesh, Envoy
	// terminates the doctor's mTLS and the backend sees an empty Subject, so
	// VerifyArtifact is rejected Unauthenticated (release.boundary_unproven A0
	// INDETERMINATE). Dialling the repo's direct host:port preserves the peer
	// certificate — same pattern as the per-node agent dials. dialOptionsFor-
	// InternalService still attaches the cluster_id-injecting interceptor, so
	// this does NOT trip forbidden_fix bypass_shared_client_for_internal_dials.
	var repoMu sync.Mutex
	var repoConn *grpc.ClientConn
	var repoEndpoint string
	col.WithRepositoryClientResolver(func() (repopb.PackageRepositoryClient, bool) {
		endpoint := config.ResolveServiceDirectAddr("repository.PackageRepository")
		if endpoint == "" {
			return nil, true
		}

		repoMu.Lock()
		defer repoMu.Unlock()

		if repoConn != nil && repoEndpoint == endpoint {
			return repopb.NewPackageRepositoryClient(repoConn), false
		}

		repoTarget := config.ResolveDialTarget(endpoint)
		conn, repoErr := grpc.NewClient(repoTarget.Address,
			dialOptionsForInternalService(repoTarget.ServerName)...)
		if repoErr != nil {
			logger.Warn("repository client refresh failed — repository invariants degraded", "endpoint", endpoint, "err", repoErr)
			if repoConn != nil {
				return repopb.NewPackageRepositoryClient(repoConn), false
			}
			return nil, false
		}
		if repoConn != nil && repoConn != conn {
			_ = repoConn.Close()
		}
		repoConn = conn
		repoEndpoint = endpoint
		return repopb.NewPackageRepositoryClient(repoConn), false
	})

	// Attach ai-memory client so the seed-integrity rule can detect drift
	// between what the active awareness bundle declares and what's actually
	// loaded into ai-memory. Optional — if unreachable, the rule falls
	// back to bundle-only verification.
	var aiMemClient ai_memorypb.AiMemoryServiceClient
	memEndpoint := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if memEndpoint != "" {
		memTarget := config.ResolveDialTarget(memEndpoint)
		if memConn, memErr := grpc.NewClient(memTarget.Address,
			dialOptionsForInternalService(memTarget.ServerName)...); memErr == nil {
			aiMemClient = ai_memorypb.NewAiMemoryServiceClient(memConn)
			col.WithAiMemoryClient(aiMemClient)
			// Persist the remediation escalation gate (a safety refusal) across
			// restart/failover via ai-memory — never etcd (EX-3). Reuses this same
			// shared client; nil-safe — if ai-memory is unreachable the gate
			// degrades to in-memory only.
			setRemediationGateAiMemoryClient(aiMemClient)
			// Persist the failure-rate audit ring (operational memory) across
			// restart/failover via ai-memory — never etcd (EX-3b). Same shared
			// client; nil-safe degradation.
			setRemediationAuditAiMemoryClient(aiMemClient)
		} else {
			logger.Warn("ai-memory client init failed — seed drift detection disabled", "err", memErr)
		}
	} else {
		logger.Info("ai-memory endpoint not in etcd — seed drift detection disabled (pre-Day-1)")
	}

	reg := rules.NewRegistry(rules.Config{
		HeartbeatStale:  cfg.heartbeatStale(),
		EmitAuditEvents: cfg.EmitAuditEvents,
	})

	naDialer := newControllerNodeAgentDialer(ccClient)
	s := &ClusterDoctorServer{
		cfg:            cfg,
		collector:      col,
		registry:       reg,
		version:        version,
		executor:       &ActionExecutor{nodeAgentDialer: naDialer},
		workflowClient: wfClient,
		clusterID:      clusterID,
		naDialer:       naDialer,
		auditStore:     rules.NewHealAuditStore(""),
	}

	// One recorder per server process. The connection is lazy, so ai-memory
	// does not need to be reachable at doctor startup — a cluster must be
	// diagnosable before its learning subsystem is available, not after.
	s.behavioralRecorder = observation.NewRecorder(observation.RecorderOptions{})

	// The governance client shares the recorder's lazy-connection posture for
	// the same reason: a cluster must be diagnosable before its governor is
	// reachable. Unreachability is a refusal at dispatch time, never a silent
	// allow — see gateRemediation.
	s.behavioralGovernor = observation.NewGovernor("", 0)

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

	// Run the healer against findings based on the requested heal mode.
	// Default (0 / OBSERVE) = classify only, no mutations.
	healMode := req.GetHealMode()
	healer := &rules.Healer{
		DryRun:     healMode != cluster_doctorpb.HealMode_HEAL_MODE_ENFORCE,
		Dispatcher: s.gatedDispatcher(),
	}
	if healMode != cluster_doctorpb.HealMode_HEAL_MODE_OBSERVE {
		healReport := healer.Evaluate(ctx, findings)
		// Persist audit trail for on-demand heal.
		if s.auditStore != nil {
			s.auditStore.AppendReport(healReport)
		}
		// Annotate each finding with its heal decision.
		for i, f := range findings {
			if i < len(healReport.Results) {
				r := healReport.Results[i]
				findings[i].HealDecisionProto = &cluster_doctorpb.HealDecision{
					Disposition: dispositionToProto(r.Disposition),
					Action:      r.Action,
					Executed:    r.Executed,
					Verified:    r.Verified,
					Error:       r.Error,
				}
			}
			_ = f // suppress unused
		}
	}
	gateSummary := appendRemediationGateEvidence(findings)

	// GetClusterReport produces the full cluster-wide finding set — it is
	// the authority for the cluster.finding.* event delta.
	s.cacheFindings(findings, true)
	report := render.ClusterReport(snap, findings, s.version, fresh)
	s.emitBehavioralClusterReport(report)
	if report.CountsByCategory == nil {
		report.CountsByCategory = map[string]uint32{}
	}
	report.CountsByCategory["remediation_gate.escalated"] = uint32(gateSummary.Escalated)
	report.CountsByCategory["remediation_gate.cooldown"] = uint32(gateSummary.Cooldown)
	return report, nil
}

// dispositionToProto maps the rules-layer disposition string to the proto enum.
func dispositionToProto(d rules.HealDisposition) cluster_doctorpb.HealDisposition {
	switch d {
	case rules.HealAuto:
		return cluster_doctorpb.HealDisposition_HEAL_AUTO
	case rules.HealPropose:
		return cluster_doctorpb.HealDisposition_HEAL_PROPOSE
	case rules.HealObserve:
		return cluster_doctorpb.HealDisposition_HEAL_OBSERVE
	}
	return cluster_doctorpb.HealDisposition_HEAL_DISPOSITION_UNSPECIFIED
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
	appendRemediationGateEvidence(findings)
	// GetNodeReport returns a subset (one node only); it must NOT update the
	// cluster-wide delta authority or emit events.
	s.cacheFindings(findings, false)

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

	why := f.Summary
	evidenceDigest := digestFindingEvidence(f.Evidence)
	historical := summarizeHistoricalSuccessfulActions(context.Background(), f.InvariantID, evidenceDigest, 200)
	if hint := historicalActionsHint(historical); hint != "" {
		why = why + " | " + hint
	}

	planDiff := []string{}
	if len(historical) > 0 {
		planDiff = append(planDiff, "historical_success_actions_present")
	}

	return &cluster_doctorpb.FindingExplanation{
		FindingId:   f.FindingID,
		InvariantId: f.InvariantID,
		WhyFailed:   why,
		Remediation: f.Remediation,
		Evidence:    f.Evidence,
		PlanDiff:    planDiff,
	}, nil
}

// cacheFindings stores the latest findings for ExplainFinding lookups and,
// when called from a cluster-wide context (clusterWide=true), emits
// cluster.finding.created / cluster.finding.resolved events for the delta
// vs the last cluster-wide snapshot.
//
// Why the clusterWide flag exists:
//
//	GetClusterReport produces the full cluster-wide finding set (N findings).
//	GetNodeReport produces a subset (only one node's findings, K < N).
//	VerifyConvergence may be either, depending on nodeID.
//
// All three previously shared a single lastFindings cache for delta
// computation. The result was spurious "resolved" events on every
// node-scoped call (N-K findings appear to disappear) followed by spurious
// "created" events on the next cluster-wide call (the same N-K reappear).
// On a dashboard polling both endpoints, this produced 100+ events per
// minute representing 0 actual state changes.
//
// Fix: track the delta authority separately. Only cluster-wide callers
// update lastEmittedFindings and emit events; node-scoped callers only
// refresh lastFindings for ExplainFinding lookups.
func (s *ClusterDoctorServer) cacheFindings(findings []rules.Finding, clusterWide bool) {
	s.lastFindingsMu.Lock()
	// Always refresh the lookup cache so ExplainFinding can resolve the
	// finding_id the caller just observed.
	s.lastFindings = findings

	if !clusterWide {
		s.lastFindingsMu.Unlock()
		return
	}

	// Cluster-wide path: compute delta against the last cluster-wide snapshot
	// (NOT against lastFindings, which may have been overwritten by a
	// node-scoped call since the last emission).
	current := make(map[string]rules.Finding, len(findings))
	for _, f := range findings {
		current[f.FindingID] = f
	}
	prev := make(map[string]rules.Finding, len(s.lastEmittedFindings))
	for _, f := range s.lastEmittedFindings {
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
	// Replace the cluster-wide delta authority with the current snapshot.
	s.lastEmittedFindings = findings
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

// gatedDispatcher implements rules.Dispatcher by routing HealAuto findings
// through ExecuteRemediation — the single execution gate that enforces
// leader, evidence-trust, hard-blocklist, approval, cooldown, failure-rate,
// and etcd audit policies. The healer NEVER mutates cluster state directly;
// every Path B dispatch reaches this struct and traverses Path A's gates.
//
// Today's PolicyV1 has zero HealAuto rules with a non-empty AutoAction (all
// were demoted to HealPropose in Milestone 2). The dispatcher is wired
// regardless so Milestone 3 can re-promote one rule by editing the policy
// alone — the gated path is already in place. See
// docs/design/auto-healing-path-unification-patch-c.md.
type gatedDispatcher struct {
	server *ClusterDoctorServer
}

// gatedDispatcher returns the rules.Dispatcher the healer uses. Tests can
// replace the field on the Healer directly with a fake; production wiring
// always uses this gated path.
func (s *ClusterDoctorServer) gatedDispatcher() rules.Dispatcher {
	return &gatedDispatcher{server: s}
}

// Dispatch routes a single HealAuto finding through ExecuteRemediation.
// Returns (executed, auditID, err). A finding with no structured
// RemediationAction is recorded as a proposal (false, "", nil) — the gate
// cannot verify what it cannot type-check.
func (g *gatedDispatcher) Dispatch(ctx context.Context, f rules.Finding, autoAction string, dryRun bool) (bool, string, error) {
	if g.server == nil {
		return false, "", fmt.Errorf("gatedDispatcher: server is nil")
	}
	// The finding must carry at least one RemediationStep with a structured
	// RemediationAction; the dispatcher picks the first such step. Rules
	// that mix text-only and actionable steps put the actionable one first
	// by convention (see artifact_integrity.go: actionStep(1, …) followed
	// by step(2, …) text fallback).
	stepIndex := -1
	for i, st := range f.Remediation {
		if st.GetAction() != nil {
			stepIndex = i
			break
		}
	}
	if stepIndex < 0 {
		logger.Info("gated-dispatcher: skipping — no structured RemediationAction on any step",
			"invariant_id", f.InvariantID,
			"entity_ref", f.EntityRef,
			"auto_action", autoAction)
		return false, "", nil
	}

	// Ask behavioral governance BEFORE the executor runs.
	//
	// The gate was wired into the WORKFLOW path only (workflow_runner.go
	// GateAction), and this — the autonomous healer's path — reached
	// executeRemediationForFinding without ever consulting it. Verified live on
	// 2026-08-01: with principle.cluster.restart_drifted_unit_with_observed_finding
	// promoted specifically to govern SYSTEMCTL_RESTART on
	// node.systemd.units_running, the healer performed exactly that action and
	// behavioral_memory.action_checks recorded zero rows from cluster_doctor. The
	// operator path was governed; the unattended one was not, which is the wrong
	// way round.
	//
	// Placed here rather than inside executeRemediationForFinding because the
	// public ExecuteRemediation RPC also routes there, and the workflow actor has
	// already gated by that point — gating again would mint a second decision for
	// one action. gatedDispatcher.Dispatch is the healer's exclusive entry, which
	// makes it the mirror of the engine's own placement: immediately before the
	// executor call.
	allowed, actionCheckID := g.server.dispatchAllowedByGovernance(ctx, f, stepIndex, autoAction)
	if !allowed {
		return false, "", nil
	}

	// The healer already holds the resolved Finding — execute against it
	// directly instead of clobbering the shared lastFindings cache with a
	// one-element slice and round-tripping through finding_id resolution. A
	// concurrent GetClusterReport/GetNodeReport could otherwise overwrite that
	// cache between the write and ExecuteRemediation's read, so remediation
	// could act on the wrong finding or spuriously NotFound.
	// (meta.code.local_state_must_not_become_hidden_authority)
	resp, err := g.server.executeRemediationForFinding(ctx, f, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: f.FindingID,
		StepIndex: uint32(stepIndex),
		DryRun:    dryRun,
	})
	if err != nil {
		return false, "", err
	}

	// Close the learning loop on the AUTONOMOUS path.
	//
	// ObserveOutcome was wired into the workflow runner only (workflow_runner.go),
	// so every repair the healer performed itself recorded nothing. The learning
	// loop reads those outcomes, so its support count for this theme stayed at
	// zero permanently — and ErrInsufficientSupport is silent by design, making
	// "never recorded" indistinguishable from "not enough yet". No candidate could
	// ever be queued, so no principle could ever be promoted, so no dispatch could
	// ever be governed, so every repair took the ungoverned fall-through. A closed
	// loop with no way in.
	//
	// Verified live on 2026-08-02: two successful restart_drifted_unit repairs of
	// globular-log.service (audits rem-1785646…, 04:25:03 and 04:49:04), both
	// EXECUTED with the unit returning to active, produced zero promotion
	// candidates at candidateMinRepeats=2.
	//
	// This is the same producer/consumer split as 2e019d9e ("the healer gated on
	// evidence it never produced"), one layer up: the healer learned from outcomes
	// it never recorded. That fix closed the evidence hop and left this one open.
	//
	// Recorded ONLY after verification, never at dispatch — an outcome written at
	// dispatch records an intention as a result (workflow_runner.go states the same
	// rule for the workflow path). Dry runs are excluded: a rehearsal is not an
	// outcome, and counting it would let observe-mode cycles manufacture support
	// for promoting an action nobody ever performed.
	if resp.GetExecuted() && !dryRun {
		// Stamped the instant the executor accepted, and nowhere else — the same
		// rule and the same placement the workflow actor uses
		// (actors_doctor_remediation.go stamps dispatched_at only when
		// res.Executed). It must never be derived from verification time:
		// post-action verification is only meaningful relative to when the action
		// actually went out, and DispatchedAt is what lets the verification prove
		// it read post-repair state.
		dispatchedAt := time.Now()

		// node_id comes from the action's params, the same source
		// dispatchAllowedByGovernance and the workflow runner both read it from.
		// Finding itself carries no node identity — EntityRef is not NodeID.
		nodeID := f.Remediation[stepIndex].GetAction().GetParams()["node_id"]
		g.server.observeHealerOutcome(ctx, f, nodeID, actionCheckID, resp.GetAuditId(), dispatchedAt)
	}

	return resp.GetExecuted(), resp.GetAuditId(), nil
}

// observeHealerOutcome verifies whether the healer's repair actually cleared the
// finding, then records the result through the same two calls the workflow path
// uses (emitBehavioralRemediationOutcome → recordGovernedOutcome). One recorder,
// two callers: the workflow actor and the healer.
//
// Convergence is re-derived from a fresh snapshot rather than assumed from
// Executed. rules.HealResult sets Verified=true whenever a dispatch executed,
// which is dispatch acknowledgement, not proof of repair — the same conflation
// reconcile.terminal_success_requires_observed_convergence forbids on the
// controller side. A repair that executed and did NOT clear the finding must be
// recorded as a FAILED outcome, because that is exactly the evidence a future
// promotion decision needs most.
//
// Best-effort by contract: a verification that cannot be taken records nothing
// rather than guessing. Losing one observation costs a slower promotion; writing
// an unverified one corrupts the support count that governs future autonomy.
func (s *ClusterDoctorServer) observeHealerOutcome(ctx context.Context, f rules.Finding, nodeID, actionCheckID, auditID string, dispatchedAt time.Time) {
	if s.collector == nil {
		logger.Warn("learning: healer outcome not recorded — no collector configured",
			"invariant_id", f.InvariantID, "entity_ref", f.EntityRef)
		return
	}

	// Post-action verification must read state collected AFTER the repair.
	//
	// GetSnapshot serves the cache, so it could return the very pre-repair
	// snapshot the finding was raised from. The finding is still present in that
	// data, so a successful repair records as FindingResolved=false — a failed
	// outcome. That is worse than recording nothing: at candidateMinRepeats the
	// learning loop would accumulate support for the proposition that this repair
	// does not work, and the promotion decision it governs would be made on
	// inverted evidence.
	//
	// Forcing fresh is necessary but NOT sufficient. GetSnapshotWithFreshness
	// invalidates the cache, but a caller can still join an already in-flight
	// fetch that STARTED BEFORE the dispatch; that returns CacheHit=false while
	// describing pre-repair state. So freshness is proven by ordering, not by the
	// cache flag: GeneratedAt must post-date the dispatch instant.
	//
	// takeSnapshot carries the leader rule (only the leader may force-fresh). If
	// leadership was lost between dispatch and verification it serves cached, the
	// ordering check below rejects it, and nothing is recorded — which is the
	// documented contract: a verification that cannot be taken records nothing
	// rather than guessing.
	snap, _, err := s.takeSnapshot(ctx, cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH)
	if err != nil && snap == nil {
		logger.Warn("learning: healer outcome not recorded — verification snapshot unavailable",
			"invariant_id", f.InvariantID, "entity_ref", f.EntityRef, "err", err)
		return
	}
	if !verificationSnapshotIsPostAction(snap, dispatchedAt) {
		logger.Warn("learning: healer outcome not recorded — no post-action snapshot available",
			"invariant_id", f.InvariantID, "entity_ref", f.EntityRef,
			"dispatched_at", dispatchedAt, "snapshot_generated_at", snapGeneratedAt(snap))
		return
	}

	// Node-scoped where possible: evaluating the whole cluster to verify one
	// unit is wasteful, and EvaluateAll would also overwrite the shared
	// cluster-wide findings cache from a path that is not authoritative for it.
	var findings []rules.Finding
	if nodeID != "" {
		findings = s.registry.EvaluateForNode(snap, nodeID)
	} else {
		findings = s.registry.EvaluateAll(snap)
	}

	stillPresent := false
	for _, cur := range findings {
		if cur.FindingID == f.FindingID {
			stillPresent = true
			break
		}
	}

	s.observeOutcome(ctx, remediation.Outcome{
		FindingID:       f.FindingID,
		ActionCheckID:   actionCheckID,
		Dispatched:      true,
		DispatchedAt:    dispatchedAt,
		Verified:        true,
		FindingResolved: !stillPresent,
		VerifiedAt:      time.Now(),
		ClusterID:       s.clusterID,
		InvariantID:     f.InvariantID,
		EntityRef:       f.EntityRef,
		NodeID:          nodeID,
		// WorkflowRunID is still empty, and this outcome is therefore still
		// LineageMissingWorkflowRun — see the open question on #236. Synthesising
		// a run id would forge lineage for a dispatch that never had one, so the
		// gap is left visible rather than papered over.
	}, auditID)
}

// verificationSnapshotIsPostAction reports whether snap may serve as post-action
// evidence for a dispatch made at dispatchedAt.
//
// Strictly After, not After-or-equal: a snapshot generated at the same instant
// the action went out does not prove it observed the result of that action.
// Rejecting the boundary costs one delayed observation; accepting it can record
// a successful repair as failed.
func verificationSnapshotIsPostAction(snap *collector.Snapshot, dispatchedAt time.Time) bool {
	return snap != nil && snap.GeneratedAt.After(dispatchedAt)
}

// snapGeneratedAt is log-only: a nil snapshot must not panic the warn path that
// exists precisely because the snapshot was unusable.
func snapGeneratedAt(snap *collector.Snapshot) time.Time {
	if snap == nil {
		return time.Time{}
	}
	return snap.GeneratedAt
}

// observeOutcome is the single recorder both remediation paths share.
func (s *ClusterDoctorServer) observeOutcome(ctx context.Context, o remediation.Outcome, auditID string) {
	evidenceID := s.emitBehavioralRemediationOutcome(o)
	s.recordGovernedOutcome(ctx, o, evidenceID)
	logger.Info("learning: healer outcome recorded",
		"invariant_id", o.InvariantID,
		"entity_ref", o.EntityRef,
		"finding_resolved", o.FindingResolved,
		"audit_id", auditID,
	)
}

// dispatchAllowedByGovernance answers whether behavioral governance permits this
// auto-heal, and reports false for every answer that is not a clear yes.
//
// A refusal is returned as "do not execute" rather than as an error: the healer
// records a non-executed dispatch as a proposal, which is what a governed
// refusal IS — the gate did its job and the finding stays open for an operator.
// Returning an error would instead count toward the cycle's MaxFailures circuit
// breaker, so a governor that is merely strict would look like an executor that
// is broken and would halt healing for unrelated findings.
//
// An unreachable governor is also a refusal, never consent
// (observation.Governor.CheckAction is explicit that a caller must not be able to
// mistake the two). This does pause auto-healing while behavioral memory is down.
// That is deliberate and bounded: the cluster's deterministic convergence — the
// controller reconciling desired state — is untouched, so "AI is supplementary,
// never required" still holds for the mechanism that guarantees the cluster
// converges. What pauses is the doctor's autonomy, which is the part that
// depends on being authorized.
func (s *ClusterDoctorServer) dispatchAllowedByGovernance(ctx context.Context, f rules.Finding, stepIndex int, autoAction string) (bool, string) {
	action := f.Remediation[stepIndex].GetAction()
	verdict, err := s.gateRemediation(ctx, engine.GateRequest{
		FindingID:   f.FindingID,
		ClusterID:   s.clusterID,
		InvariantID: f.InvariantID,
		EntityRef:   f.EntityRef,
		NodeID:      action.GetParams()["node_id"],
		ActionKind:  action.GetActionType().String(),
		StepIndex:   uint32(stepIndex),
		// No WorkflowRunID and no ApprovalToken: this dispatch belongs to no
		// workflow run and carries no operator approval. Both are stated by
		// omission rather than filled with a plausible value — a fabricated run
		// id would forge lineage, and a fabricated token would tell the governor
		// a human approved something no human saw.
	})
	if err != nil {
		logger.Warn("gated-dispatcher: REFUSED — behavioral governor unavailable; auto-heal paused for this finding",
			"invariant_id", f.InvariantID,
			"entity_ref", f.EntityRef,
			"auto_action", autoAction,
			"err", err,
		)
		return false, ""
	}
	if !verdict.Allowed {
		logger.Info("gated-dispatcher: REFUSED by behavioral governance",
			"invariant_id", f.InvariantID,
			"entity_ref", f.EntityRef,
			"auto_action", autoAction,
			"action_check_id", verdict.ActionCheckID,
			"status", verdict.Status,
			"reason", verdict.Reason,
			"principles", verdict.PrincipleIDs,
		)
		return false, verdict.ActionCheckID
	}
	if verdict.Governed {
		logger.Info("gated-dispatcher: allowed by behavioral governance",
			"invariant_id", f.InvariantID,
			"entity_ref", f.EntityRef,
			"action_check_id", verdict.ActionCheckID,
			"principles", verdict.PrincipleIDs,
		)
	}
	// Ungoverned-and-allowed proceeds: no promoted principle applies, so the
	// action keeps exactly the protection it had before governance existed (the
	// executor's leader, risk-class, approval, unit-allowlist and cooldown
	// gates). gateRemediation has already counted the coverage gap.
	//
	// The ActionCheckID is returned even when UNGOVERNED. gateRemediation still
	// minted a check row recording that this action ran with no applicable
	// principle, and recordGovernedOutcome drops any outcome whose ActionCheckID
	// is empty — so discarding it here would silently throw away every outcome
	// the autonomous healer produces, which is precisely the gap this path exists
	// to close.
	return true, verdict.ActionCheckID
}

// GetHealHistory returns recent heal action records from the persistent audit trail.
func (s *ClusterDoctorServer) GetHealHistory(ctx context.Context, req *cluster_doctorpb.GetHealHistoryRequest) (*cluster_doctorpb.GetHealHistoryResponse, error) {
	if s.auditStore == nil {
		return &cluster_doctorpb.GetHealHistoryResponse{}, nil
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	records, err := s.auditStore.ReadHistory(rules.HealHistoryFilter{
		Node:         req.GetNode(),
		Package:      req.GetPackageName(),
		InvariantID:  req.GetInvariantId(),
		ExecutedOnly: req.GetExecutedOnly(),
		FailuresOnly: req.GetFailuresOnly(),
		Limit:        limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read heal history: %v", err)
	}
	resp := &cluster_doctorpb.GetHealHistoryResponse{
		Total: int32(len(records)),
	}
	for _, r := range records {
		resp.Records = append(resp.Records, &cluster_doctorpb.HealHistoryRecord{
			Ts:          r.Timestamp.Format(time.RFC3339),
			CycleId:     r.CycleID,
			InvariantId: r.InvariantID,
			EntityRef:   r.EntityRef,
			Node:        r.Node,
			PackageName: r.Package,
			Disposition: string(r.Disposition),
			Action:      r.Action,
			Executed:    r.Executed,
			Verified:    r.Verified,
			Error:       r.Error,
		})
	}
	return resp, nil
}
