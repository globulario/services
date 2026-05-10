package collector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// CollectorConfig carries per-fetch settings.
type CollectorConfig struct {
	ListTimeout time.Duration
	NodeTimeout time.Duration
	Concurrency int
	SnapshotTTL time.Duration
}

// Collector gathers upstream state and maintains a SnapshotCache.
type Collector struct {
	cfg                 CollectorConfig
	controllerClient    cluster_controllerpb.ClusterControllerServiceClient
	workflowClient      workflowpb.WorkflowServiceClient
	repoClient          repopb.PackageRepositoryClient // optional; nil until WithRepositoryClient
	repoEndpointMissing bool                           // true when etcd has no entry for repository.PackageRepository
	clusterID           string
	cache               *SnapshotCache

	connMu     sync.Mutex
	agentConns map[string]*grpc.ClientConn // keyed by AgentEndpoint

	// Prometheus access
	promEndpoint  string
	promTokenFile string
	promInsecure  bool

	// driftSince tracks when each node's services hash drift was first observed.
	// Used to compute NodeDriftAge in each Snapshot for severity escalation.
	driftMu    sync.Mutex
	driftSince map[string]time.Time // keyed by nodeID
}

func New(cfg CollectorConfig, cc cluster_controllerpb.ClusterControllerServiceClient) *Collector {
	return &Collector{
		cfg:              cfg,
		controllerClient: cc,
		cache:            NewSnapshotCache(cfg.SnapshotTTL),
		agentConns:       make(map[string]*grpc.ClientConn),
		driftSince:       make(map[string]time.Time),
		promEndpoint:     defaultPromEndpoint(),
		promTokenFile:    os.Getenv("PROMETHEUS_BEARER_FILE"),
		promInsecure:     os.Getenv("PROMETHEUS_INSECURE") == "1",
	}
}

// WithWorkflowClient attaches a workflow-service client so the collector can
// pull convergence telemetry (step outcomes, drift, run summaries) into the
// snapshot. Optional — if nil, telemetry-based invariants degrade gracefully.
func (c *Collector) WithWorkflowClient(wf workflowpb.WorkflowServiceClient, clusterID string) *Collector {
	c.workflowClient = wf
	c.clusterID = clusterID
	return c
}

// WithRepositoryClient attaches a repository-service client so the collector
// can pull ListRepositoryFindings and GetRepositoryStatus into the snapshot.
// Optional — if nil, repository invariants produce no findings.
func (c *Collector) WithRepositoryClient(repo repopb.PackageRepositoryClient) *Collector {
	c.repoClient = repo
	return c
}

// SetRepositoryEndpointMissing records that the repository service endpoint
// was not found in etcd at startup. The flag is propagated into each Snapshot
// so the repositoryOperationalMode invariant can emit a repository.endpoint_missing
// finding once the cluster is past bootstrap (nodes visible in controller).
func (c *Collector) SetRepositoryEndpointMissing() {
	c.repoEndpointMissing = true
}

// SnapshotResult carries the snapshot plus provenance telemetry the
// report layer needs to populate the freshness fields in ReportHeader.
// CacheHit is true when the cache already had a fresh entry and no
// upstream fetch was performed for this call. CacheTTL is the TTL the
// cache is currently configured with; callers surface it so operators
// know the maximum staleness a cached response can have.
type SnapshotResult struct {
	Snapshot *Snapshot
	CacheHit bool
	CacheTTL time.Duration
}

// GetSnapshot returns a cached or freshly fetched Snapshot. Kept for
// back-compat; new callers should prefer GetSnapshotWithFreshness,
// which also reports whether the response was a cache hit.
func (c *Collector) GetSnapshot(ctx context.Context) (*Snapshot, error) {
	res, err := c.GetSnapshotWithFreshness(ctx, false)
	if res.Snapshot != nil || err == nil {
		return res.Snapshot, err
	}
	return nil, err
}

// GetSnapshotWithFreshness is the freshness-aware fetch entry point.
// When forceFresh is true the cached snapshot is dropped before the
// fetch so the caller is guaranteed an authoritative read — useful
// right after running a remediation, or when opening an incident.
//
// The returned SnapshotResult always carries CacheTTL so callers can
// communicate the staleness window to operators without reaching into
// doctor's config. CacheHit is decided on THIS call's path, not on
// any concurrent caller that may be waiting behind a singleflight.
func (c *Collector) GetSnapshotWithFreshness(ctx context.Context, forceFresh bool) (SnapshotResult, error) {
	res := SnapshotResult{CacheTTL: c.cache.ttlFor()}
	if forceFresh {
		c.cache.invalidate()
	}
	cached, waiter := c.cache.get()
	if cached != nil {
		res.Snapshot = cached
		res.CacheHit = true
		return res, nil
	}
	if waiter != nil {
		// Another goroutine is already fetching — share its result.
		// From this caller's point of view an upstream fetch DID
		// happen (we did not return cached bytes without waiting),
		// so CacheHit stays false.
		select {
		case snap := <-waiter:
			res.Snapshot = snap
			return res, nil
		case <-ctx.Done():
			return res, ctx.Err()
		}
	}

	// We are the fetcher.
	snap, err := c.fetch(ctx)
	if err != nil {
		// On hard error still store a partial snapshot so callers get what we have.
		if snap == nil {
			snap = newSnapshot(uuid.New().String())
			snap.DataIncomplete = true
		}
	}
	c.cache.set(snap)
	res.Snapshot = snap
	return res, err
}

// fetch does the actual upstream calls.
func (c *Collector) fetch(ctx context.Context) (*Snapshot, error) {
	snap := newSnapshot(uuid.New().String())
	snap.RepositoryEndpointMissing = c.repoEndpointMissing

	// ── 1. ListNodes ──────────────────────────────────────────────────────────
	listCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	nodesResp, err := c.controllerClient.ListNodes(listCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		snap.addError("cluster_controller", "ListNodes", err)
	} else {
		snap.Nodes = nodesResp.GetNodes()
		snap.addSource("cluster_controller.ListNodes")
	}

	// ── 2. GetClusterHealthV1 ─────────────────────────────────────────────────
	healthCtx, cancel2 := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel2()

	healthResp, err := c.controllerClient.GetClusterHealthV1(healthCtx, &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		snap.addError("cluster_controller", "GetClusterHealthV1", err)
	} else {
		for _, nh := range healthResp.GetNodes() {
			snap.NodeHealths[nh.GetNodeId()] = nh
		}
		snap.addSource("cluster_controller.GetClusterHealthV1")
	}

	// Track how long each node has been in a services-hash drift state.
	// First observation records the start time; cleared when drift resolves.
	snap.NodeDriftAge = c.updateDriftSince(snap.NodeHealths)

	// ── 2b. NodePackageKinds — package kind per node from etcd ───────────────
	// Key format: /globular/nodes/{nodeID}/packages/{KIND}/{name}
	// We do a single prefix scan per known node and parse the kind from the path.
	// This is the authoritative source for package classification; rules must
	// use this instead of hardcoded name lists so new packages work automatically.
	if len(snap.Nodes) > 0 {
		if etcdCli, err := config.GetEtcdClient(); err == nil {
			pkgCtx, pkgCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
			defer pkgCancel()
			for _, node := range snap.Nodes {
				nid := node.GetNodeId()
				if nid == "" {
					continue
				}
				prefix := "/globular/nodes/" + nid + "/packages/"
				resp, err := etcdCli.Get(pkgCtx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
				if err != nil {
					continue
				}
				kinds := make(map[string]string, len(resp.Kvs))
				for _, kv := range resp.Kvs {
					// key = /globular/nodes/{nid}/packages/{KIND}/{name}
					tail := strings.TrimPrefix(string(kv.Key), prefix)
					slash := strings.Index(tail, "/")
					if slash <= 0 || slash == len(tail)-1 {
						continue
					}
					kind := tail[:slash]
					name := tail[slash+1:]
					if kind != "" && name != "" {
						kinds[name] = strings.ToUpper(kind)
					}
				}
				if len(kinds) > 0 {
					snap.mu.Lock()
					snap.NodePackageKinds[nid] = kinds
					snap.mu.Unlock()
				}
			}
			snap.addSource("etcd.node_package_kinds")
		}
	}

	// ── 3. ObjectStoreDesiredState — objectstore topology from etcd ──────────
	osCtx, osCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer osCancel()
	if desired, err := config.LoadObjectStoreDesiredState(osCtx); err != nil {
		snap.addError("etcd", "LoadObjectStoreDesiredState", err)
		snap.ObjectStoreDesiredLoadError = err
	} else if desired != nil {
		snap.ObjectStoreDesired = desired
		snap.addSource("etcd.objectstore_desired_state")
	}

	// ── 3a. ObjectStoreAppliedGeneration — last applied topology generation ──
	if cli, err := config.GetEtcdClient(); err == nil {
		applyCtx, applyCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer applyCancel()
		if resp, err := cli.Get(applyCtx, config.EtcdKeyObjectStoreAppliedGeneration); err == nil && len(resp.Kvs) > 0 {
			if gen, err := strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64); err == nil {
				snap.ObjectStoreAppliedGeneration = gen
				snap.addSource("etcd.objectstore_applied_generation")
			}
		}
	}

	// ── 3b. Per-node rendered generation + fingerprint — from etcd ──────────
	// Collected for all nodes (not just MinIO pool) so invariants can check
	// render readiness without needing a separate RPC hop.
	if len(snap.Nodes) > 0 {
		if etcdCli, err := config.GetEtcdClient(); err == nil {
			renderCtx, renderCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
			defer renderCancel()
			for _, node := range snap.Nodes {
				nid := node.GetNodeId()
				if nid == "" {
					continue
				}
				// rendered_generation
				genKey := config.EtcdKeyNodeRenderedGeneration(nid)
				if genResp, err := etcdCli.Get(renderCtx, genKey); err == nil && len(genResp.Kvs) > 0 {
					if gen, err := strconv.ParseInt(string(genResp.Kvs[0].Value), 10, 64); err == nil {
						snap.mu.Lock()
						snap.NodeRenderedGenerations[nid] = gen
						snap.mu.Unlock()
					}
				}
				// rendered_state_fingerprint
				fpKey := config.EtcdKeyNodeRenderedStateFingerprint(nid)
				if fpResp, err := etcdCli.Get(renderCtx, fpKey); err == nil && len(fpResp.Kvs) > 0 {
					snap.mu.Lock()
					snap.NodeRenderedFingerprints[nid] = string(fpResp.Kvs[0].Value)
					snap.mu.Unlock()
				}
			}
			snap.addSource("etcd.node_rendered_generations")
		}
	}

	// ── 3d. CAMetadata — CA fingerprint from etcd ────────────────────────────
	caCtx, caCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer caCancel()
	if caMeta, err := config.LoadCAMetadata(caCtx); err != nil {
		snap.addError("etcd", "LoadCAMetadata", err)
	} else if caMeta != nil {
		snap.CAMetadata = caMeta
		snap.addSource("etcd.pki_ca_metadata")
	}

	// ── 3d2. Ingress desired-state + node ingress status ─────────────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		ingCtx, ingCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer ingCancel()
		if resp, err := etcdCli.Get(ingCtx, "/globular/ingress/v1/spec"); err != nil {
			snap.addError("etcd", "Get(/globular/ingress/v1/spec)", err)
			snap.IngressSpecLoadError = err
		} else if len(resp.Kvs) > 0 {
			snap.IngressSpecPresent = true
			snap.IngressSpecRaw = string(resp.Kvs[0].Value)
			snap.addSource("etcd.ingress_spec")
		}

		if resp, err := etcdCli.Get(ingCtx, "/globular/ingress/v1/status/", clientv3.WithPrefix()); err == nil {
			for _, kv := range resp.Kvs {
				key := string(kv.Key)
				nodeID := strings.TrimPrefix(key, "/globular/ingress/v1/status/")
				if nodeID == "" {
					continue
				}
				var payload map[string]interface{}
				if err := json.Unmarshal(kv.Value, &payload); err != nil {
					continue
				}
				snap.mu.Lock()
				snap.IngressNodeStatus[nodeID] = payload
				snap.mu.Unlock()
			}
			snap.addSource("etcd.ingress_status")
		}
	}

	// ── 3e. Admitted disks — operator-approved disk records from etcd ────────
	admitCtx, admitCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer admitCancel()
	if admitted, err := config.LoadAdmittedDisks(admitCtx); err != nil {
		snap.addError("etcd", "LoadAdmittedDisks", err)
	} else {
		snap.AdmittedDisks = admitted
		snap.addSource("etcd.admitted_disks")
	}

	// ── 3f. Disk candidates — per-node inventory from etcd ───────────────────
	diskCtx, diskCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer diskCancel()
	if candidates, err := config.LoadAllDiskCandidates(diskCtx); err != nil {
		snap.addError("etcd", "LoadAllDiskCandidates", err)
	} else {
		snap.DiskCandidates = candidates
		snap.addSource("etcd.disk_candidates")
	}

	// ── 3g. Applied state fingerprint + volumes hash ──────────────────────────
	if cli, err := config.GetEtcdClient(); err == nil {
		fpCtx, fpCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer fpCancel()
		if resp, err := cli.Get(fpCtx, config.EtcdKeyObjectStoreAppliedStateFingerprint); err == nil && len(resp.Kvs) > 0 {
			snap.AppliedStateFingerprint = string(resp.Kvs[0].Value)
			snap.addSource("etcd.objectstore_applied_state_fingerprint")
		}
		if resp, err := cli.Get(fpCtx, config.EtcdKeyObjectStoreAppliedVolumesHash); err == nil && len(resp.Kvs) > 0 {
			snap.AppliedVolumesHash = string(resp.Kvs[0].Value)
		}
	}

	// ── 3h. Desired topology transition record (destructive change guard) ─────
	if snap.ObjectStoreDesired != nil {
		tCtx, tCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer tCancel()
		if transition, err := config.LoadTopologyTransition(tCtx, snap.ObjectStoreDesired.Generation); err == nil && transition != nil {
			snap.DesiredTopologyTransition = transition
			snap.addSource("etcd.topology_transition")
		}
	}

	// ── 3i. Scylla schema-guard status from controller ───────────────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		sgCtx, sgCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer sgCancel()
		if resp, err := etcdCli.Get(sgCtx, "/globular/scylla/schema_guard/", clientv3.WithPrefix()); err == nil {
			for _, kv := range resp.Kvs {
				key := string(kv.Key)
				keyspace := strings.TrimPrefix(key, "/globular/scylla/schema_guard/")
				if keyspace == "" {
					continue
				}
				var payload map[string]interface{}
				if err := json.Unmarshal(kv.Value, &payload); err != nil {
					continue
				}
				snap.mu.Lock()
				snap.ScyllaSchemaGuardStatus[keyspace] = payload
				snap.mu.Unlock()
			}
			snap.addSource("etcd.scylla_schema_guard")
		}
	}

	// ── 3j. DNS zone reload status ────────────────────────────────────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		dnsCtx, dnsCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer dnsCancel()
		if resp, err := etcdCli.Get(dnsCtx, "/globular/dns/v1/status"); err == nil && len(resp.Kvs) > 0 {
			var payload map[string]interface{}
			if err := json.Unmarshal(resp.Kvs[0].Value, &payload); err == nil {
				snap.DNSZoneReloadStatus = payload
				snap.addSource("etcd.dns_zone_reload_status")
			}
		}
	}

	// ── 3k. Controller reconcile lane statuses ────────────────────────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		laneCtx, laneCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer laneCancel()
		if resp, err := etcdCli.Get(laneCtx, "/globular/controller/reconcile/lanes/", clientv3.WithPrefix()); err == nil {
			for _, kv := range resp.Kvs {
				key := string(kv.Key)
				lane := strings.TrimPrefix(key, "/globular/controller/reconcile/lanes/")
				if lane == "" {
					continue
				}
				var payload map[string]interface{}
				if err := json.Unmarshal(kv.Value, &payload); err != nil {
					continue
				}
				snap.mu.Lock()
				snap.ReconcileLaneStatus[lane] = payload
				snap.mu.Unlock()
			}
			snap.addSource("etcd.controller_reconcile_lanes")
		}
	}

	// ── 3k-2. Per-package kind mismatch records ──────────────────────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		kmCtx, kmCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer kmCancel()
		if resp, err := etcdCli.Get(kmCtx, "/globular/controller/kind_mismatches/", clientv3.WithPrefix()); err == nil {
			for _, kv := range resp.Kvs {
				var rec KindMismatchRecord
				if err := json.Unmarshal(kv.Value, &rec); err != nil {
					continue
				}
				snap.mu.Lock()
				snap.KindMismatches = append(snap.KindMismatches, rec)
				snap.mu.Unlock()
			}
			snap.addSource("etcd.kind_mismatches")
		}
	}

	// ── 3k-3. Controller leader pending-update record ────────────────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		lpCtx, lpCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer lpCancel()
		if resp, err := etcdCli.Get(lpCtx, "/globular/controller/leader_pending_update"); err == nil && len(resp.Kvs) > 0 {
			var rec LeaderPendingUpdateRecord
			if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err == nil {
				snap.mu.Lock()
				snap.LeaderPendingUpdate = &rec
				snap.mu.Unlock()
				snap.addSource("etcd.leader_pending_update")
			}
		}
	}

	// ── 3l. Critical key presence checks (Case 05 doctor wiring) ─────────────
	if etcdCli, err := config.GetEtcdClient(); err == nil {
		keyCtx, keyCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
		defer keyCancel()
		for _, key := range config.CriticalEtcdKeys {
			if resp, err := etcdCli.Get(keyCtx, key); err != nil {
				snap.CriticalKeyQueryError[key] = err
			} else {
				snap.CriticalKeyPresent[key] = len(resp.Kvs) > 0
			}
		}
		for _, prefix := range config.CriticalEtcdPrefixes {
			if resp, err := etcdCli.Get(keyCtx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(1)); err != nil {
				snap.CriticalKeyQueryError[prefix] = err
			} else {
				snap.CriticalKeyPresent[prefix] = len(resp.Kvs) > 0
			}
		}
		snap.addSource("etcd.critical_key_presence")
	}

	// Static ownership-completeness check — no etcd query required.
	// Detects when a key is added to CriticalEtcdKeys/Prefixes without a
	// corresponding CriticalKeyPolicy entry in the config package.
	snap.CriticalKeyPolicyGaps = config.PolicyGapsForKeys(config.CriticalEtcdKeys, config.CriticalEtcdPrefixes)

	// ── 4. Per-node calls (concurrent, capped) ────────────────────────────────
	if len(snap.Nodes) > 0 {
		c.fetchPerNode(ctx, snap)
	}

	// ── 5. Workflow convergence telemetry (optional) ─────────────────────────
	if c.workflowClient != nil && c.clusterID != "" {
		c.fetchWorkflowTelemetry(ctx, snap)
	}

	// ── 6. Repository findings + operational status (optional) ───────────────
	if c.repoClient != nil {
		c.fetchRepositoryData(ctx, snap)
	}

	// ── 7. Prometheus control-plane signals (best-effort) ───────────────────
	c.fetchPrometheus(ctx, snap)

	return snap, nil
}

// fetchWorkflowTelemetry pulls bounded convergence signals from the workflow
// service: per-step outcomes, drift items still unresolved, and per-workflow
// summaries. These drive the workflow.* invariants.
func (c *Collector) fetchWorkflowTelemetry(ctx context.Context, snap *Snapshot) {
	wfCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	// Inject cluster_id into outgoing gRPC metadata so the workflow
	// service's interceptor doesn't reject the call with
	// "cluster_id required after cluster initialization". The same
	// metadata pattern is used by node_agent's artifact.fetch and
	// cluster_controller's release_resolver.
	if c.clusterID != "" {
		md := metadata.Pairs("cluster_id", c.clusterID)
		wfCtx = metadata.NewOutgoingContext(wfCtx, md)
	}

	if stepsResp, err := c.workflowClient.ListStepOutcomes(wfCtx, &workflowpb.ListStepOutcomesRequest{
		ClusterId: c.clusterID,
	}); err != nil {
		snap.addError("workflow", "ListStepOutcomes", err)
	} else {
		snap.StepOutcomes = stepsResp.GetOutcomes()
		snap.addSource("workflow.ListStepOutcomes")
	}

	if summariesResp, err := c.workflowClient.ListWorkflowSummaries(wfCtx, &workflowpb.ListWorkflowSummariesRequest{
		ClusterId: c.clusterID,
	}); err != nil {
		snap.addError("workflow", "ListWorkflowSummaries", err)
	} else {
		snap.WorkflowSummaries = summariesResp.GetSummaries()
		snap.addSource("workflow.ListWorkflowSummaries")
	}

	if driftResp, err := c.workflowClient.ListDriftUnresolved(wfCtx, &workflowpb.ListDriftUnresolvedRequest{
		ClusterId: c.clusterID,
	}); err != nil {
		snap.addError("workflow", "ListDriftUnresolved", err)
	} else {
		snap.DriftUnresolved = driftResp.GetItems()
		snap.addSource("workflow.ListDriftUnresolved")
	}

	// MC-4: Fetch blocked runs (paused for operator approval).
	if blockedResp, err := c.workflowClient.ListRuns(wfCtx, &workflowpb.ListRunsRequest{
		ClusterId: c.clusterID,
		Status:    workflowpb.RunStatus_RUN_STATUS_BLOCKED,
		Limit:     20,
	}); err != nil {
		snap.addError("workflow", "ListRuns(BLOCKED)", err)
	} else {
		snap.BlockedRuns = blockedResp.GetRuns()
		snap.addSource("workflow.ListRuns(BLOCKED)")
	}

	// WF-DEFER B3: fetch correlations that have been auto-abandoned
	// after hitting max_defers. Each surfaces as a doctor finding
	// requiring operator action (clear via WorkflowService).
	if abResp, err := c.workflowClient.ListCorrelationDeferState(wfCtx, &workflowpb.ListCorrelationDeferStateRequest{
		ClusterId:     c.clusterID,
		AbandonedOnly: true,
	}); err != nil {
		snap.addError("workflow", "ListCorrelationDeferState(abandoned)", err)
	} else {
		snap.AbandonedDeferCorrelations = abResp.GetRecords()
		snap.addSource("workflow.ListCorrelationDeferState(abandoned)")
	}
}

func (c *Collector) fetchPerNode(ctx context.Context, snap *Snapshot) {
	sem := make(chan struct{}, c.cfg.Concurrency)
	var wg sync.WaitGroup

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		endpoint := node.GetAgentEndpoint()
		if endpoint == "" {
			snap.addError("node_agent@"+nodeID, "dial", fmt.Errorf("node %s has no agent_endpoint", nodeID))
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(nid, ep string) {
			defer wg.Done()
			defer func() { <-sem }()

			agentClient, err := c.agentClient(ep)
			if err != nil {
				snap.addError("node_agent@"+nid, "dial", err)
				return
			}

			// GetInventory
			invCtx, cancel := context.WithTimeout(ctx, c.cfg.NodeTimeout)
			defer cancel()
			invResp, err := agentClient.GetInventory(invCtx, &node_agentpb.GetInventoryRequest{})
			if err != nil {
				snap.addError("node_agent@"+nid, "GetInventory", err)
			} else {
				snap.mu.Lock()
				snap.Inventories[nid] = invResp.GetInventory()
				snap.mu.Unlock()
				snap.addSource("node_agent.GetInventory@" + nid)
			}

			// GetSubsystemHealth — read background goroutine health.
			shCtx, shCancel := context.WithTimeout(ctx, c.cfg.NodeTimeout)
			shResp, shErr := agentClient.GetSubsystemHealth(shCtx, &node_agentpb.GetSubsystemHealthRequest{})
			shCancel()
			if shErr != nil {
				if !strings.Contains(shErr.Error(), "Unimplemented") {
					snap.addError("node_agent@"+nid, "GetSubsystemHealth", shErr)
				}
			} else {
				snap.mu.Lock()
				snap.SubsystemHealth[nid] = shResp
				snap.mu.Unlock()
				snap.addSource("node_agent.GetSubsystemHealth@" + nid)
			}

			// GetCertificateStatus — read cert SANs, expiry, chain validity.
			certCtx, certCancel := context.WithTimeout(ctx, c.cfg.NodeTimeout)
			certResp, certErr := agentClient.GetCertificateStatus(certCtx, &node_agentpb.GetCertificateStatusRequest{})
			certCancel()
			if certErr != nil {
				if !strings.Contains(certErr.Error(), "Unimplemented") {
					snap.addError("node_agent@"+nid, "GetCertificateStatus", certErr)
				}
			} else {
				snap.mu.Lock()
				snap.CertificateStatus[nid] = certResp
				snap.mu.Unlock()
				snap.addSource("node_agent.GetCertificateStatus@" + nid)
			}

			// VerifyPackageIntegrity — reads installed_state, the local
			// artifact cache, and the repository manifest. Read-only.
			//
			// Timeout is intentionally much larger than NodeTimeout
			// because the action makes one GetArtifactManifest RPC per
			// installed package (~48 packages × ~100ms each → 5s+ wall
			// time). With the default 5-second NodeTimeout, the call
			// reliably times out and the invariant silently sees no
			// data. The tighter GetInventory deadline is fine; this
			// call needs its own budget.
			integTimeout := c.cfg.NodeTimeout * 6
			if integTimeout < 30*time.Second {
				integTimeout = 30 * time.Second
			}
			log.Printf("collector: VerifyPackageIntegrity on node=%s endpoint=%s timeout=%s",
				nid, ep, integTimeout)
			integCtx, cancel2 := context.WithTimeout(ctx, integTimeout)
			defer cancel2()
			start := time.Now()
			integResp, ierr := agentClient.VerifyPackageIntegrity(integCtx, &node_agentpb.VerifyPackageIntegrityRequest{
				NodeId: nid,
			})
			elapsed := time.Since(start)
			switch {
			case ierr != nil:
				// Unimplemented is the "old node_agent" sentinel — log
				// at info level without marking the snapshot incomplete.
				if strings.Contains(ierr.Error(), "Unimplemented") {
					log.Printf("collector: VerifyPackageIntegrity on node=%s not supported (old binary), skipping: %v",
						nid, ierr)
				} else {
					// Everything else is a real failure the operator
					// should see. Surface it to the snapshot error
					// stream (→ data_incomplete) AND log the full
					// message including elapsed time so timeout vs.
					// auth vs. dial issues are distinguishable.
					log.Printf("collector: VerifyPackageIntegrity on node=%s FAILED after %s: %v",
						nid, elapsed, ierr)
					snap.addError("node_agent@"+nid, "VerifyPackageIntegrity", ierr)
				}
			case !integResp.GetOk():
				// Server-side failure: action returned ok=false. The
				// handler populates error_detail in this branch.
				log.Printf("collector: VerifyPackageIntegrity on node=%s returned ok=false after %s: %s",
					nid, elapsed, integResp.GetErrorDetail())
				snap.addError("node_agent@"+nid, "VerifyPackageIntegrity",
					fmt.Errorf("ok=false: %s", integResp.GetErrorDetail()))
			case integResp.GetReportJson() == "":
				log.Printf("collector: VerifyPackageIntegrity on node=%s returned empty report after %s",
					nid, elapsed)
				snap.addError("node_agent@"+nid, "VerifyPackageIntegrity", fmt.Errorf("empty report_json"))
			default:
				var report IntegrityReport
				if uerr := json.Unmarshal([]byte(integResp.GetReportJson()), &report); uerr != nil {
					log.Printf("collector: VerifyPackageIntegrity on node=%s JSON parse failed after %s: %v",
						nid, elapsed, uerr)
					snap.addError("node_agent@"+nid, "VerifyPackageIntegrity",
						fmt.Errorf("parse report_json: %w", uerr))
				} else {
					snap.mu.Lock()
					snap.IntegrityReports[nid] = &report
					snap.mu.Unlock()
					snap.addSource("node_agent.VerifyPackageIntegrity@" + nid)
					log.Printf("collector: VerifyPackageIntegrity on node=%s stored report (checked=%d, findings=%d) in %s",
						nid, report.Checked, len(report.Findings), elapsed)
				}
			}

			// Plan collection removed — plan system deleted.
		}(nodeID, endpoint)
	}

	wg.Wait()
}

// fetchRepositoryData pulls ListRepositoryFindings and GetRepositoryStatus from
// the repository service. Both are best-effort: a failure populates the
// RepositoryOperationalStatus.ReachError so the invariant can emit a
// "repository.unreachable" finding without marking the whole snapshot incomplete.
func (c *Collector) fetchRepositoryData(ctx context.Context, snap *Snapshot) {
	repoCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	// GetRepositoryStatus — must answer even when Scylla is down.
	statusResp, statusErr := c.repoClient.GetRepositoryStatus(repoCtx, &repopb.GetRepositoryStatusRequest{})
	opStatus := &RepositoryOperationalStatus{ReachError: statusErr}
	if statusErr == nil && statusResp != nil {
		opStatus.Service = statusResp.GetService()
		opStatus.Mode = statusResp.GetMode()
		opStatus.Reason = statusResp.GetReason()
		opStatus.ObservedAtUnix = statusResp.GetObservedAtUnix()
		for _, d := range statusResp.GetDependencies() {
			opStatus.Dependencies = append(opStatus.Dependencies, RepoDependencyHealth{
				Name:                d.GetName(),
				Kind:                d.GetKind(),
				Status:              d.GetStatus(),
				Reason:              d.GetReason(),
				AffectsCapabilities: d.GetAffectsCapabilities(),
			})
		}
		for _, c := range statusResp.GetCapabilities() {
			opStatus.Capabilities = append(opStatus.Capabilities, RepoCapabilityHealth{
				Name:   c.GetName(),
				Status: c.GetStatus(),
				Mode:   c.GetMode(),
				Reason: c.GetReason(),
			})
		}
		snap.addSource("repository.GetRepositoryStatus")
	} else if statusErr != nil {
		snap.addError("repository", "GetRepositoryStatus", statusErr)
	}
	snap.mu.Lock()
	snap.RepositoryOperationalStatus = opStatus
	snap.mu.Unlock()

	// ListRepositoryFindings — integrity findings from the repository catalog.
	findCtx, findCancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer findCancel()

	findResp, findErr := c.repoClient.ListRepositoryFindings(findCtx, &repopb.ListRepositoryFindingsRequest{})
	if findErr != nil {
		snap.addError("repository", "ListRepositoryFindings", findErr)
		return
	}
	var findings []*RepositoryFindingSnapshot
	for _, f := range findResp.GetFindings() {
		if f == nil {
			continue
		}
		ev := make(map[string]string, len(f.GetEvidence()))
		for k, v := range f.GetEvidence() {
			ev[k] = v
		}
		findings = append(findings, &RepositoryFindingSnapshot{
			Kind:               f.GetKind().String(),
			Severity:           f.GetSeverity().String(),
			ArtifactKey:        f.GetArtifactKey(),
			PublisherID:        f.GetRef().GetPublisherId(),
			Name:               f.GetRef().GetName(),
			Version:            f.GetRef().GetVersion(),
			Platform:           f.GetRef().GetPlatform(),
			CurrentState:       f.GetCurrentState(),
			ExpectedState:      f.GetExpectedState(),
			Reason:             f.GetReason(),
			RecommendedCommand: f.GetRecommendedCommand(),
			Evidence:           ev,
			ObservedAtUnix:     f.GetObservedAtUnix(),
		})
	}
	snap.mu.Lock()
	snap.RepositoryFindings = findings
	snap.mu.Unlock()
	snap.addSource("repository.ListRepositoryFindings")
}

// agentClient returns a cached or new NodeAgent gRPC client for the given endpoint.
func (c *Collector) agentClient(endpoint string) (node_agentpb.NodeAgentServiceClient, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if conn, ok := c.agentConns[endpoint]; ok {
		return node_agentpb.NewNodeAgentServiceClient(conn), nil
	}

	dt := config.ResolveDialTarget(endpoint)
	conn, err := grpc.NewClient(dt.Address, grpc.WithTransportCredentials(agentClientTLSCreds(dt.ServerName)))
	if err != nil {
		return nil, fmt.Errorf("dial node agent %s: %w", dt.Address, err)
	}
	c.agentConns[endpoint] = conn
	return node_agentpb.NewNodeAgentServiceClient(conn), nil
}

// agentClientTLSCreds returns gRPC transport credentials for dialling node agents.
// See buildClientTLSCreds in server.go for the loopback ServerName rationale;
// this mirrors that behavior for per-node dials.
func agentClientTLSCreds(serverName string) credentials.TransportCredentials {
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
	// Load mTLS client cert so downstream RPCs that require an
	// authenticated peer identity (e.g. node_agent.VerifyPackageIntegrity
	// with permission=read) see a real principal, not anonymous.
	clientCert := "/var/lib/globular/pki/issued/services/service.crt"
	clientKey := "/var/lib/globular/pki/issued/services/service.key"
	if cert, err := tls.LoadX509KeyPair(clientCert, clientKey); err == nil {
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsCfg)
}

// Close releases all agent connections.
// updateDriftSince updates the in-memory drift-start map and returns a
// NodeDriftAge map suitable for inclusion in a Snapshot.
// Nodes with matching desired/applied hashes have their entry cleared.
// Nodes with mismatching hashes get their first-seen timestamp recorded (or preserved).
func (c *Collector) updateDriftSince(healths map[string]*cluster_controllerpb.NodeHealth) map[string]time.Duration {
	now := time.Now()
	ages := make(map[string]time.Duration)

	c.driftMu.Lock()
	defer c.driftMu.Unlock()

	// Collect set of node IDs currently in drift.
	inDrift := make(map[string]bool)
	for nodeID, nh := range healths {
		desired := nh.GetDesiredServicesHash()
		applied := nh.GetAppliedServicesHash()
		if desired == "" || desired == "services:none" || desired == applied {
			continue
		}
		inDrift[nodeID] = true
		if _, seen := c.driftSince[nodeID]; !seen {
			c.driftSince[nodeID] = now
		}
		ages[nodeID] = now.Sub(c.driftSince[nodeID])
	}

	// Clear nodes that have converged.
	for nodeID := range c.driftSince {
		if !inDrift[nodeID] {
			delete(c.driftSince, nodeID)
		}
	}

	return ages
}

func (c *Collector) Close() {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	for _, conn := range c.agentConns {
		conn.Close()
	}
	c.agentConns = make(map[string]*grpc.ClientConn)
}
