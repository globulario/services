package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/storage_backend"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// DescribePackage aggregates repository catalog + etcd desired-state + per-node
// installed state into a single flat PackageInfo. Live aggregator — no
// ScyllaDB cache. See projection-clauses.md and docs/architecture/
// cluster-introspection-plan.md for the contract.
//
// Flow (all fan-outs execute concurrently for p99 latency):
//  1. Walk the artifact catalog for every version with name matching
//     `name` (or its `-`/`_` sibling). Kind is whichever the catalog knows.
//  2. Read desired-state from etcd. Kind-dispatch:
//     - SERVICE        → /globular/resources/ServiceDesiredVersion/<name>
//     - INFRASTRUCTURE → /globular/resources/InfrastructureRelease/<pub>/<name>
//     - COMMAND        → no desired entry (commands are direct-install)
//  3. Prefix-scan /globular/nodes/*/packages/*/<name> in etcd for per-node
//     installed state.
//
// Result buckets:
//   - installed_on: nodes reporting status="installed" with any version
//   - failing_on:   nodes reporting status="failed" or "pending"
//
// Clause 6 (Size): <3 KB for clusters up to ~30 nodes.
func (srv *server) DescribePackage(ctx context.Context, req *repopb.DescribePackageRequest) (*repopb.DescribePackageResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	publisher := strings.TrimSpace(req.GetPublisherId())

	// Try both name separators — catalog publishers are inconsistent.
	candidates := []string{name}
	if alt := strings.ReplaceAll(name, "-", "_"); alt != name {
		candidates = append(candidates, alt)
	}
	if alt := strings.ReplaceAll(name, "_", "-"); alt != name {
		candidates = append(candidates, alt)
	}

	// Fan out catalog + etcd scans concurrently.
	type catalogResult struct {
		versions   []string
		latest     string
		kind       repopb.ArtifactKind
		storedKind repopb.ArtifactKind // kind as recorded in the artifact manifest (before inferCorrectKind)
		publisher  string
	}
	catalogCh := make(chan catalogResult, 1)
	installedCh := make(chan map[string][]installedRow, 1)
	desiredCh := make(chan *repopb.DesiredInfo, 1)

	var wg sync.WaitGroup
	wg.Add(3)

	// 1. Catalog walk.
	go func() {
		defer wg.Done()
		catalogCh <- srv.walkCatalogFor(ctx, candidates, publisher)
	}()

	// 2. Installed state scan.
	go func() {
		defer wg.Done()
		installedCh <- scanInstalledState(ctx, candidates)
	}()

	// 3. Desired state — requires knowing the kind; use the catalog result.
	// We can't block here, so we race: fetch BOTH possible desired paths and
	// let the merge step pick the one that matches the catalog kind.
	go func() {
		defer wg.Done()
		desiredCh <- fetchDesired(ctx, name, publisher)
	}()

	wg.Wait()
	cat := <-catalogCh
	installed := <-installedCh
	desired := <-desiredCh

	if cat.kind == repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED && len(installed) == 0 && !desired.GetPresent() {
		return nil, status.Errorf(codes.NotFound, "no package %q in repository or cluster state", name)
	}

	// Build per-node tuples. Merge catalog-derived kind's bucket with
	// whatever nodes actually have in their per-node packages/<KIND>/<name>.
	info := &repopb.PackageInfo{
		Name:          name,
		Kind:          cat.kind,
		Publisher:     cat.publisher,
		Versions:      cat.versions,
		LatestVersion: cat.latest,
		Desired:       desired,
		Source:        buildSource(cat.storedKind, cat.kind),
		ObservedAt:    time.Now().Unix(),
	}
	if publisher != "" && info.Publisher == "" {
		info.Publisher = publisher
	}

	// Flatten per-node records — callers chain node_id → node_resolve for
	// hostname display (Clause 12: no hidden coupling).
	//
	// Dedup by (node_id, version): a node may have BOTH packages/SERVICE/<name>
	// and packages/INFRASTRUCTURE/<name> entries if the kind was set wrong at
	// some point (e.g. yesterday's claude/envoy mess). We collapse them here
	// so one node shows once — the catalog kind is the truth, stale mismatches
	// are a separate hygiene problem.
	seen := make(map[string]bool)
	for _, rows := range installed {
		for _, r := range rows {
			key := r.NodeID + "|" + r.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			entry := &repopb.NodeInstallation{
				NodeId:      r.NodeID,
				Version:     r.Version,
				Status:      r.Status,
				Checksum:    r.Checksum,
				InstalledAt: r.InstalledAt,
			}
			switch strings.ToLower(r.Status) {
			case "installed":
				info.InstalledOn = append(info.InstalledOn, entry)
			case "failed", "pending", "error":
				info.FailingOn = append(info.FailingOn, entry)
			default:
				// Unknown status — treat as installed if version present.
				if r.Version != "" {
					info.InstalledOn = append(info.InstalledOn, entry)
				}
			}
		}
	}

	return &repopb.DescribePackageResponse{Info: info}, nil
}

// installedRow is the decoded /globular/nodes/<id>/packages/<KIND>/<name>
// JSON document. Fields mirror the node-agent's install report.
type installedRow struct {
	NodeID      string `json:"nodeId"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Checksum    string `json:"checksum"`
	Kind        string `json:"kind"`
	InstalledAt int64  `json:"installedUnix,string"`
}

// walkCatalogFor scans the repository's artifact manifests for any file
// whose name matches one of the candidate names. Returns the full version
// set (as semver strings — comprehensive forensic inventory), the highest
// INSTALLABLE-BY-PIN version as `latest`, the kind, and the publisher.
//
// `latest` is filtered through repopb.IsInstallableByPin so a YANKED /
// QUARANTINED / REVOKED / ARCHIVED / CORRUPTED / FAILED / ORPHANED build
// can never appear as the package's latest version in operator output —
// matching the semantics already enforced by the resolver and by the
// cluster-doctor's RepositoryVersionIndex (collector.go:1034-1038).
// `versions` keeps every observed version (installable or not) since it
// serves as a forensic inventory in `globular repository explain-package`.
//
// If multiple publishers publish the same name, the first one observed wins
// (catalog conflicts are a separate problem).
func (srv *server) walkCatalogFor(ctx context.Context, candidates []string, publisherFilter string) (r struct {
	versions   []string
	latest     string
	kind       repopb.ArtifactKind
	storedKind repopb.ArtifactKind // kind as recorded in the artifact manifest (before inferCorrectKind)
	publisher  string
}) {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return r
	}
	seen := make(map[string]struct{})
	// versionInstallable tracks whether ANY build at a given version is
	// installable-by-pin. A single version can have multiple builds
	// (different build_numbers / platforms / publishers); the version is
	// considered installable as long as at least one of those builds is.
	versionInstallable := make(map[string]bool)
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil {
			continue
		}
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		if !nameMatchesAny(ref.GetName(), candidates) {
			continue
		}
		if publisherFilter != "" && !strings.EqualFold(ref.GetPublisherId(), publisherFilter) {
			continue
		}
		v := ref.GetVersion()
		if v != "" {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				r.versions = append(r.versions, v)
			}
			if repopb.IsInstallableByPin(state) {
				versionInstallable[v] = true
			}
		}
		if r.kind == repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
			// Fill a missing aggregated kind from the stored manifest kind, which is
			// registry-authoritative since Slice 4a stamps it at publish/sync. Slice 4b
			// trusts the stored kind — no read-time inferCorrectKind correction (a live
			// audit proved every stored manifest already carries the correct kind).
			r.storedKind = rawManifestKind(ctx, srv.Storage(), manifestStorageKey(key))
			r.kind = r.storedKind
		}
		if r.publisher == "" {
			r.publisher = ref.GetPublisherId()
		}
	}
	if len(r.versions) > 0 {
		sortSemverDesc(r.versions)
		r.latest = pickLatestInstallable(r.versions, versionInstallable)
	}
	return r
}

// pickLatestInstallable returns the first (highest) entry in versionsDesc
// that is marked installable in versionInstallable. Returns "" when no
// installable version exists — callers should display "no installable
// version" rather than fall back to a non-installable one.
//
// Pure helper — exported within the package for unit testing without
// requiring a storage backend.
func pickLatestInstallable(versionsDesc []string, versionInstallable map[string]bool) string {
	for _, v := range versionsDesc {
		if versionInstallable[v] {
			return v
		}
	}
	return ""
}

func nameMatchesAny(have string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(have, c) {
			return true
		}
	}
	return false
}

// scanInstalledState gathers per-node installation rows for every
// candidate name by walking the cluster's nodes via two typed RPCs:
// cluster_controller.ListNodes (to enumerate nodes + their agent
// endpoints) and node_agent.ListInstalledPackages on each node (the
// owner of L3 installed state).
//
// Replaces the prior raw /globular/nodes/*/packages/*/<name> etcd
// scan — owned by node_agent, never by the repository server — per
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
//
// Degraded-read semantics: if any node is unreachable (no agent
// endpoint, dial failure, or RPC failure), this function logs a
// structured warning naming the node and the reason, then proceeds
// without rows for that node. It MUST NOT fabricate canonical
// installed truth from partial observations — the returned map is the
// best-effort view of the nodes the repository could actually reach.
// The caller's PackageInfo simply shows fewer NodeInstallation entries
// when nodes are unreachable; operators see the warning in repository
// logs.
func scanInstalledState(ctx context.Context, candidates []string) map[string][]installedRow {
	out := make(map[string][]installedRow)

	// Resolve controller endpoint via the service registry. An empty
	// address means we degrade to "no cluster context observable" —
	// the existing behaviour of the prior etcd-error path.
	addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
	if addr == "" {
		slog.Warn("repository.scanInstalledState: cluster_controller endpoint unresolved — installed-state view is empty")
		return out
	}
	ccTarget := config.ResolveDialTarget(addr)
	ccCreds, err := repositoryClientTLSCreds(ccTarget.ServerName)
	if err != nil {
		slog.Warn("repository.scanInstalledState: TLS creds load failed — installed-state view is empty",
			"addr", ccTarget.Address, "err", err)
		return out
	}
	ccConn, err := grpc.NewClient(ccTarget.Address, grpc.WithTransportCredentials(ccCreds))
	if err != nil {
		slog.Warn("repository.scanInstalledState: dial cluster_controller failed — installed-state view is empty",
			"addr", ccTarget.Address, "err", err)
		return out
	}
	defer func() { _ = ccConn.Close() }()

	listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
	ctrl := cluster_controllerpb.NewClusterControllerServiceClient(ccConn)
	nodesResp, err := ctrl.ListNodes(listCtx, &cluster_controllerpb.ListNodesRequest{})
	listCancel()
	if err != nil {
		slog.Warn("repository.scanInstalledState: ListNodes failed — installed-state view is empty",
			"err", err)
		return out
	}

	for _, node := range nodesResp.GetNodes() {
		nid := node.GetNodeId()
		if nid == "" {
			continue
		}
		endpoint := strings.TrimSpace(node.GetAgentEndpoint())
		if endpoint == "" {
			slog.Warn("repository.scanInstalledState: node has no agent_endpoint — installed state not observed",
				"node", nid)
			continue
		}

		naTarget := config.ResolveDialTarget(endpoint)
		naCreds, cErr := repositoryClientTLSCreds(naTarget.ServerName)
		if cErr != nil {
			slog.Warn("repository.scanInstalledState: TLS creds load failed — installed state not observed",
				"node", nid, "endpoint", endpoint, "err", cErr)
			continue
		}
		naConn, dErr := grpc.NewClient(naTarget.Address, grpc.WithTransportCredentials(naCreds))
		if dErr != nil {
			slog.Warn("repository.scanInstalledState: dial node_agent failed — installed state not observed",
				"node", nid, "endpoint", endpoint, "err", dErr)
			continue
		}

		pkgCtx, pkgCancel := context.WithTimeout(ctx, 3*time.Second)
		agent := node_agentpb.NewNodeAgentServiceClient(naConn)
		pkgResp, lpErr := agent.ListInstalledPackages(pkgCtx, &node_agentpb.ListInstalledPackagesRequest{})
		pkgCancel()
		_ = naConn.Close()
		if lpErr != nil {
			slog.Warn("repository.scanInstalledState: ListInstalledPackages failed — installed state not observed",
				"node", nid, "endpoint", endpoint, "err", lpErr)
			continue
		}

		for _, pkg := range pkgResp.GetPackages() {
			name := pkg.GetName()
			if name == "" || !nameMatchesAny(name, candidates) {
				continue
			}
			row := installedRow{
				NodeID:      nid,
				Name:        name,
				Version:     pkg.GetVersion(),
				Status:      pkg.GetStatus(),
				Checksum:    pkg.GetChecksum(),
				Kind:        pkg.GetKind(),
				InstalledAt: pkg.GetInstalledUnix(),
			}
			out[nid] = append(out[nid], row)
		}
	}
	return out
}

// extractNodeIDFromKey is retained for unit-test backwards
// compatibility; the runtime path no longer parses etcd keys.
func extractNodeIDFromKey(key string) string {
	const prefix = "/globular/nodes/"
	if !strings.HasPrefix(key, prefix) {
		return ""
	}
	rest := key[len(prefix):]
	if i := strings.Index(rest, "/"); i > 0 {
		return rest[:i]
	}
	return ""
}

// fetchDesired reads the cluster-wide desired-state pointer for a package
// via the cluster_controller's typed GetDesiredState RPC. Returns a
// DesiredInfo with present=false when no entry exists (e.g. COMMAND
// packages).
//
// The publisherFilter is preserved for the public signature but is now
// best-effort: GetDesiredState's flat DesiredService list is keyed by
// canonical service_id (the controller's listAllDesiredServices strips
// publisher prefixes), so an exact per-publisher disambiguation is no
// longer carried in the typed response. If two infra publishers ever
// shipped the same component name, this call returns whichever the
// controller merged first.
//
// DesiredInfo.Generation was sourced from the etcd record's
// meta.generation field. GetDesiredState does not surface that field
// (purely cosmetic — only pkg_info CLI displays "(gen %d)"). Set to
// zero; consumers must not treat it as authoritative.
//
// Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc
func fetchDesired(ctx context.Context, name, _ string) *repopb.DesiredInfo {
	// NOTE: every failure path below returns Present:false, which is
	// shape-identical to "no desired record exists". The accompanying WARN
	// logs preserve the diagnostic for operators. Adding a typed
	// "Observable" / "Reason" field on DesiredInfo would let callers
	// distinguish unreachable from absent without a log lookup — tracked
	// as defense-in-depth (meta.fallback_must_degrade_semantics).
	absent := &repopb.DesiredInfo{Present: false}
	addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
	if addr == "" {
		slog.Warn("repository.fetchDesired: controller endpoint unresolved — returning Present:false (operator: check controller discovery)",
			"package", name)
		return absent
	}
	target := config.ResolveDialTarget(addr)
	creds, err := repositoryClientTLSCreds(target.ServerName)
	if err != nil {
		slog.Warn("repository.fetchDesired: TLS creds load failed — returning Present:false (operator: check repository PKI)",
			"addr", target.Address, "package", name, "err", err)
		return absent
	}
	conn, err := grpc.NewClient(target.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		slog.Warn("repository.fetchDesired: dial controller failed — returning Present:false (operator: check controller reachability)",
			"addr", target.Address, "package", name, "err", err)
		return absent
	}
	defer func() { _ = conn.Close() }()

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	resp, err := client.GetDesiredState(callCtx, &emptypb.Empty{})
	if err != nil {
		return absent
	}
	for _, svc := range resp.GetServices() {
		if !strings.EqualFold(svc.GetServiceId(), name) {
			continue
		}
		if svc.GetVersion() == "" {
			continue
		}
		return &repopb.DesiredInfo{
			Version: svc.GetVersion(),
			Present: true,
		}
	}
	return absent
}

// parseDesiredServiceVersion / parseDesiredInfraRelease /
// extractInfraPublisher were removed in v1.2.175. fetchDesired now
// consumes the typed GetDesiredState RPC; the JSON parsers and the
// publisher-prefix extractor that fed the prior raw-etcd path are
// dead code.

// sortSemverDesc sorts a slice of version strings descending. Accepts
// non-semver strings too (falls back to lexicographic).
func sortSemverDesc(vs []string) {
	// Lightweight: lexicographic sort works for monotonic "a.b.c" version
	// strings the catalog uses. Swap for semver.Compare if the repo ever
	// admits non-monotonic versions.
	for i := 1; i < len(vs); i++ {
		for j := i; j > 0 && compareVersions(vs[j], vs[j-1]) > 0; j-- {
			vs[j], vs[j-1] = vs[j-1], vs[j]
		}
	}
}

// rawManifestKind reads the raw manifest JSON bytes directly from storage and
// extracts the ref.kind field WITHOUT applying inferCorrectKind. This captures
// the kind as it was recorded at publish time ("stored kind"). Used only by
// walkCatalogFor to detect when inferCorrectKind changed the kind, so the
// operator-visible explain output can show both stored and effective values.
func rawManifestKind(ctx context.Context, st storage_backend.Storage, storageKey string) repopb.ArtifactKind {
	data, err := st.ReadFile(ctx, storageKey)
	if err != nil {
		return repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED
	}
	var raw struct {
		Ref struct {
			Kind string `json:"kind"`
		} `json:"ref"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED
	}
	if v, ok := repopb.ArtifactKind_value[raw.Ref.Kind]; ok {
		return repopb.ArtifactKind(v)
	}
	return repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED
}

// buildSource constructs the Source field for PackageInfo. When inferCorrectKind
// changed the kind (storedKind != effectiveKind and stored is not UNSPECIFIED),
// it appends a "; kind-normalized: STORED→EFFECTIVE" suffix so CLI can surface
// the warning without proto changes.
func buildSource(stored, effective repopb.ArtifactKind) string {
	const base = "live-aggregator"
	unspecified := repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED
	if stored != unspecified && stored != effective {
		return base + "; kind-normalized: " + stored.String() + "→" + effective.String()
	}
	return base
}

func compareVersions(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) || i < len(bp); i++ {
		var ai, bi int
		if i < len(ap) {
			fmt.Sscanf(ap[i], "%d", &ai)
		}
		if i < len(bp) {
			fmt.Sscanf(bp[i], "%d", &bi)
		}
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}
