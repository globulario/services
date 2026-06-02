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

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/render"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
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

	// Attach repository-service client for ListRepositoryFindings + GetRepositoryStatus.
	// Resolved from etcd; optional — if unreachable, repository invariants degrade gracefully.
	repoEndpoint := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoEndpoint != "" {
		repoTarget := config.ResolveDialTarget(repoEndpoint)
		if repoConn, repoErr := grpc.NewClient(repoTarget.Address,
			grpc.WithTransportCredentials(buildClientTLSCreds(repoTarget.ServerName))); repoErr == nil {
			col.WithRepositoryClient(repopb.NewPackageRepositoryClient(repoConn))
		} else {
			logger.Warn("repository client init failed — repository invariants disabled", "err", repoErr)
		}
	} else {
		logger.Info("repository endpoint not in etcd — repository invariants disabled (pre-bootstrap)")
		col.SetRepositoryEndpointMissing()
	}

	// Attach ai-memory client so the seed-integrity rule can detect drift
	// between what the active awareness bundle declares and what's actually
	// loaded into ai-memory. Optional — if unreachable, the rule falls
	// back to bundle-only verification.
	var aiMemClient ai_memorypb.AiMemoryServiceClient
	memEndpoint := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if memEndpoint != "" {
		memTarget := config.ResolveDialTarget(memEndpoint)
		if memConn, memErr := grpc.NewClient(memTarget.Address,
			grpc.WithTransportCredentials(buildClientTLSCreds(memTarget.ServerName))); memErr == nil {
			aiMemClient = ai_memorypb.NewAiMemoryServiceClient(memConn)
			col.WithAiMemoryClient(aiMemClient)
		} else {
			logger.Warn("ai-memory client init failed — seed drift detection disabled", "err", memErr)
		}
	} else {
		logger.Info("ai-memory endpoint not in etcd — seed drift detection disabled (pre-Day-1)")
	}

	// Attach awareness-graph client so the collector can probe the RDF store
	// and detect an empty graph (embedded seed failed or Oxigraph was wiped
	// after startup). Optional — if unreachable, no findings are emitted.
	awEndpoint := config.ResolveServiceAddr("awareness.AwarenessGraphService", "")
	if awEndpoint != "" {
		awTarget := config.ResolveDialTarget(awEndpoint)
		if awConn, awErr := grpc.NewClient(awTarget.Address,
			grpc.WithTransportCredentials(buildClientTLSCreds(awTarget.ServerName))); awErr == nil {
			col.WithAwarenessGraphClient(awarenesspb.NewAwarenessGraphClient(awConn))
		} else {
			logger.Warn("awareness-graph client init failed — seed_empty detection disabled", "err", awErr)
		}
	} else {
		logger.Info("awareness-graph endpoint not in etcd — seed_empty detection disabled (pre-Day-1)")
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
	// The finding must carry a structured RemediationAction at step 0 for
	// ExecuteRemediation to dispatch. Today no PolicyV1 HealAuto rule does;
	// Milestone 3 re-promotes one rule whose corresponding invariant emits
	// a typed action via actionStep(...).
	if len(f.Remediation) == 0 || f.Remediation[0].GetAction() == nil {
		logger.Info("gated-dispatcher: skipping — no structured RemediationAction on finding",
			"invariant_id", f.InvariantID,
			"entity_ref", f.EntityRef,
			"auto_action", autoAction)
		return false, "", nil
	}
	// Populate the last-findings cache so ExecuteRemediation can resolve
	// finding_id back to the Finding object. The healer cycle's scope is
	// authoritative (cluster-wide=true) when invoked by the leader loop.
	g.server.cacheFindings([]rules.Finding{f}, false)

	resp, err := g.server.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: f.FindingID,
		StepIndex: 0,
		DryRun:    dryRun,
	})
	if err != nil {
		return false, "", err
	}
	return resp.GetExecuted(), resp.GetAuditId(), nil
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
