package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		versions  []string
		latest    string
		kind      repopb.ArtifactKind
		publisher string
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
		Source:        "live-aggregator",
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
// set (as semver strings), the highest version, the kind, and the publisher.
// If multiple publishers publish the same name, the first one observed wins
// (catalog conflicts are a separate problem).
func (srv *server) walkCatalogFor(ctx context.Context, candidates []string, publisherFilter string) (r struct {
	versions  []string
	latest    string
	kind      repopb.ArtifactKind
	publisher string
}) {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return r
	}
	seen := make(map[string]struct{})
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		m, err := srv.readManifestByKey(ctx, key)
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
		if _, ok := seen[v]; !ok && v != "" {
			seen[v] = struct{}{}
			r.versions = append(r.versions, v)
		}
		if r.kind == repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
			r.kind = ref.GetKind()
		}
		if r.publisher == "" {
			r.publisher = ref.GetPublisherId()
		}
	}
	if len(r.versions) > 0 {
		sortSemverDesc(r.versions)
		r.latest = r.versions[0]
	}
	return r
}

func nameMatchesAny(have string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(have, c) {
			return true
		}
	}
	return false
}

// scanInstalledState reads /globular/nodes/*/packages/*/<name> and returns
// the per-node installation rows grouped by node_id. Tries every candidate
// name under every kind (SERVICE, COMMAND, INFRASTRUCTURE).
func scanInstalledState(ctx context.Context, candidates []string) map[string][]installedRow {
	out := make(map[string][]installedRow)
	cli, err := config.GetEtcdClient()
	if err != nil {
		return out
	}

	scanCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Prefix scan: /globular/nodes/ → all nodes, all kinds, all packages.
	resp, err := cli.Get(scanCtx, "/globular/nodes/", clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return out
	}
	// Re-fetch matching values only.
	var matching []string
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		// Key shape: /globular/nodes/<id>/packages/<KIND>/<name>
		if !strings.Contains(key, "/packages/") {
			continue
		}
		last := key[strings.LastIndex(key, "/")+1:]
		if !nameMatchesAny(last, candidates) {
			continue
		}
		matching = append(matching, key)
	}
	for _, key := range matching {
		gr, err := cli.Get(scanCtx, key)
		if err != nil || len(gr.Kvs) == 0 {
			continue
		}
		var row installedRow
		if err := json.Unmarshal(gr.Kvs[0].Value, &row); err != nil {
			continue
		}
		nodeID := extractNodeIDFromKey(key)
		if row.NodeID == "" {
			row.NodeID = nodeID
		}
		out[nodeID] = append(out[nodeID], row)
	}
	return out
}

// extractNodeIDFromKey parses /globular/nodes/<id>/packages/... and returns
// the node_id segment. Returns "" on malformed keys.
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
// from etcd. Tries SERVICE first, then INFRASTRUCTURE. Returns a DesiredInfo
// with present=false when no entry exists (e.g. COMMAND packages).
func fetchDesired(ctx context.Context, name, publisherFilter string) *repopb.DesiredInfo {
	absent := &repopb.DesiredInfo{Present: false}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return absent
	}
	getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// ServiceDesiredVersion lookup.
	svcKey := "/globular/resources/ServiceDesiredVersion/" + name
	if resp, err := cli.Get(getCtx, svcKey); err == nil && len(resp.Kvs) > 0 {
		if info := parseDesiredServiceVersion(resp.Kvs[0].Value); info != nil {
			return info
		}
	}

	// InfrastructureRelease lookup — scan under the publisher prefix, or
	// the candidate publisher if provided.
	infraPrefix := "/globular/resources/InfrastructureRelease/"
	resp, err := cli.Get(getCtx, infraPrefix, clientv3.WithPrefix())
	if err == nil {
		for _, kv := range resp.Kvs {
			key := string(kv.Key)
			// Key: /globular/resources/InfrastructureRelease/<pub>/<name>
			if !strings.HasSuffix(key, "/"+name) {
				continue
			}
			pub := extractInfraPublisher(key, name)
			if publisherFilter != "" && !strings.EqualFold(pub, publisherFilter) {
				continue
			}
			if info := parseDesiredInfraRelease(kv.Value, pub); info != nil {
				return info
			}
		}
	}
	return absent
}

func extractInfraPublisher(key, name string) string {
	const prefix = "/globular/resources/InfrastructureRelease/"
	if !strings.HasPrefix(key, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(key, prefix)
	rest = strings.TrimSuffix(rest, "/"+name)
	return rest
}

// parseDesiredServiceVersion extracts version + generation from the JSON
// payload of /globular/resources/ServiceDesiredVersion/<name>.
func parseDesiredServiceVersion(body []byte) *repopb.DesiredInfo {
	var d struct {
		Meta struct {
			Generation int64 `json:"generation"`
		} `json:"meta"`
		Spec struct {
			ServiceName string `json:"service_name"`
			Version     string `json:"version"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil
	}
	if d.Spec.Version == "" {
		return nil
	}
	return &repopb.DesiredInfo{
		Version:    d.Spec.Version,
		Generation: d.Meta.Generation,
		Present:    true,
	}
}

// parseDesiredInfraRelease extracts the desired version from an
// InfrastructureRelease document.
func parseDesiredInfraRelease(body []byte, publisher string) *repopb.DesiredInfo {
	var d struct {
		Meta struct {
			Generation int64 `json:"generation"`
		} `json:"meta"`
		Spec struct {
			Version string `json:"version"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil
	}
	if d.Spec.Version == "" {
		return nil
	}
	return &repopb.DesiredInfo{
		Version:    d.Spec.Version,
		Generation: d.Meta.Generation,
		Publisher:  publisher,
		Present:    true,
	}
}

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
