package collector

// verification.go — Phase 9 wire-up of the Diagnostic Honesty Refactor.
//
// The verifier orchestrator in golang/verifier is pure: it takes targets +
// evidence and returns verdicts. This file is the I/O wrapper that runs
// inside the cluster_doctor collector sweep — read desired state from
// etcd, line up per-node ServiceRuntimeProofs already gathered by
// fetchPerNode, call verifier.VerifyTarget, AggregateResult, write each
// per-(node, service) result to /globular/verification/runtime/<node>/<svc>.
//
// v1.2.57 hash-schema fix (claude_fix_verifier_hash_schema_false_positive.md):
// the verifier's binary comparison surface must be fed with the binary
// entrypoint_checksum (sha256 of /usr/lib/globular/bin/<name>), NOT the
// package tarball digest. The repository's ArtifactManifest carries both
// fields; we look up the manifest by (publisher, name, version, build_number)
// and cache the result by build_id so we only pay the RPC once per release.

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/verifier"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

// fetchDesiredServiceTargets reads ServiceRelease and InfrastructureRelease
// records from etcd and folds them into snap.DesiredServiceTargets, keyed
// by canonical service name. Best-effort: a transient etcd error degrades
// the verification step rather than failing the whole sweep.
//
// ServiceRelease keys:           /globular/resources/ServiceRelease/<name>
// InfrastructureRelease keys:    /globular/resources/InfrastructureRelease/<name>
func (c *Collector) fetchDesiredServiceTargets(ctx context.Context, snap *Snapshot) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		snap.addError("etcd", "GetEtcdClient(verification.targets)", err)
		return
	}

	loadCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	// ── ServiceRelease ──
	srResp, err := cli.Get(loadCtx, "/globular/resources/ServiceRelease/", clientv3.WithPrefix())
	if err != nil {
		snap.addError("etcd", "Get(ServiceRelease/*)", err)
	} else {
		for _, kv := range srResp.Kvs {
			var rel cluster_controllerpb.ServiceRelease
			if jerr := json.Unmarshal(kv.Value, &rel); jerr != nil {
				continue
			}
			tgt := serviceReleaseToTarget(&rel, snap.Nodes)
			if tgt != nil {
				snap.DesiredServiceTargets[tgt.Service] = tgt
			}
		}
		snap.addSource("etcd.ServiceRelease/*")
	}

	// ── InfrastructureRelease ──
	irResp, err := cli.Get(loadCtx, "/globular/resources/InfrastructureRelease/", clientv3.WithPrefix())
	if err != nil {
		snap.addError("etcd", "Get(InfrastructureRelease/*)", err)
	} else {
		for _, kv := range irResp.Kvs {
			var rel cluster_controllerpb.InfrastructureRelease
			if jerr := json.Unmarshal(kv.Value, &rel); jerr != nil {
				continue
			}
			tgt := infraReleaseToTarget(&rel, snap.Nodes)
			if tgt != nil {
				snap.DesiredServiceTargets[tgt.Service] = tgt
			}
		}
		snap.addSource("etcd.InfrastructureRelease/*")
	}

	// ── Enrich every target with entrypoint_checksum from the repo ──
	// (runs after the etcd reads above so we make ONE pass over the
	// resolved set, deduplicating by build_id.)
	c.enrichTargetsWithEntrypointChecksum(ctx, snap)

	// ── Apply policy gates that override the default per-release target ──
	// shape. The ingress spec lives in /globular/ingress/v1/spec, fetched
	// earlier in the snapshot; when mode=disabled the keepalived unit must
	// NOT be running and we tell the verifier so (RuntimeNeeded=false).
	// This prevents the verifier from raising runtime_identity_unproven
	// every sweep on a Day-0 / pre-ingress cluster.
	applyIngressPolicyToTargets(snap)

	// Clear RuntimeNeeded for COMMAND-kind packages (mc, restic, ffmpeg, etc.)
	// — they have no systemd unit and no running process to prove. The default
	// RuntimeNeeded=true would generate permanent runtime_identity_unproven
	// incidents for every installed command binary.
	applyCommandKindPolicyToTargets(snap)
}

// applyIngressPolicyToTargets patches per-target expectations using the
// cluster's ingress desired state. Currently a single rule: when ingress
// is disabled, keepalived is expected NOT to run, so its target's
// RuntimeNeeded flag is cleared. The verifier's computeProofStatus
// honours that — installed-only proof becomes sufficient instead of
// firing a degraded "no runtime proof" finding every sweep.
func applyIngressPolicyToTargets(snap *Snapshot) {
	if snap == nil || !ingressDisabledFromSnapshot(snap) {
		return
	}
	if tgt := snap.DesiredServiceTargets["keepalived"]; tgt != nil {
		tgt.RuntimeNeeded = false
	}
}

// applyCommandKindPolicyToTargets clears RuntimeNeeded for any target
// whose package kind is COMMAND according to the per-node installed-state
// records in snap.NodePackageKinds.
//
// COMMAND packages (mc, restic, ffmpeg, sctool, etcdctl, etc.) are CLI
// binaries with no systemd unit and no long-running process. Keeping
// RuntimeNeeded=true for them causes the verifier to emit a permanent
// runtime_identity_unproven finding every sweep — the binary is installed
// correctly but no process matches it. This function suppresses those false
// positives by marking the target as binary-install-only.
func applyCommandKindPolicyToTargets(snap *Snapshot) {
	if snap == nil {
		return
	}
	commandPackages := make(map[string]bool)
	for _, kinds := range snap.NodePackageKinds {
		for name, kind := range kinds {
			if kind == "COMMAND" {
				commandPackages[name] = true
			}
		}
	}
	if len(commandPackages) == 0 {
		return
	}
	for name, tgt := range snap.DesiredServiceTargets {
		if tgt != nil && commandPackages[name] {
			tgt.RuntimeNeeded = false
		}
	}
}

// ingressDisabledFromSnapshot returns true when the ingress spec parsed
// from snap.IngressSpecRaw carries mode="disabled" or
// explicit_disabled=true. Conservative on read/parse failures: returns
// false so the existing fail-open behaviour is preserved (the verifier
// keeps RuntimeNeeded=true and any real keepalived outage still surfaces).
func ingressDisabledFromSnapshot(snap *Snapshot) bool {
	if snap == nil || !snap.IngressSpecPresent {
		return false
	}
	raw := strings.TrimSpace(snap.IngressSpecRaw)
	if raw == "" {
		return false
	}
	var spec struct {
		Mode             string `json:"mode"`
		ExplicitDisabled bool   `json:"explicit_disabled"`
	}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(spec.Mode), "disabled") || spec.ExplicitDisabled
}

// serviceReleaseToTarget maps a ServiceRelease object into a target.
// Returns nil for releases that have no resolved version (PENDING/WAITING).
//
// DesiredEntrypointChecksum is pre-populated from the release status when
// available (ResolvedEntrypointChecksum). enrichTargetsWithEntrypointChecksum
// then uses a repo RPC as a best-effort refresh / fallback for older releases
// that don't carry the field yet. Pre-populating here means a repo timeout
// or transient RPC failure can no longer leave the checksum empty and
// cause spurious inventory_claim verdicts on services that were already
// verified (e.g. after a node-join bumps LastTransitionUnixMs).
func serviceReleaseToTarget(rel *cluster_controllerpb.ServiceRelease, nodes []*cluster_controllerpb.NodeRecord) *DesiredServiceTarget {
	if rel == nil || rel.Spec == nil || rel.Status == nil {
		return nil
	}
	if strings.TrimSpace(rel.Status.ResolvedVersion) == "" {
		return nil // not yet resolved; nothing to verify
	}
	required := requiredNodesFromStatus(rel.Status.Nodes, nodes)
	return &DesiredServiceTarget{
		Service:                   strings.TrimSpace(rel.Spec.ServiceName),
		PublisherID:               rel.Spec.PublisherID,
		DesiredVersion:            rel.Status.ResolvedVersion,
		DesiredBuildID:            rel.Status.ResolvedBuildID,
		DesiredEntrypointChecksum: normalizeEntrypointChecksum(rel.Status.ResolvedEntrypointChecksum),
		DesiredPackageDigest:      rel.Status.ResolvedArtifactDigest, // tarball — kept separate
		RuntimeNeeded:             true,                              // ServiceRelease implies a running systemd unit
		RequiredNodes:             required,
		ApplyTime:                 lastTransitionFromStatus(rel.Status.LastTransitionUnixMs),
	}
}

// infraReleaseToTarget maps an InfrastructureRelease object into a target.
// Same shape as serviceReleaseToTarget — the verifier treats both kinds
// identically; the difference is only at apply time.
func infraReleaseToTarget(rel *cluster_controllerpb.InfrastructureRelease, nodes []*cluster_controllerpb.NodeRecord) *DesiredServiceTarget {
	if rel == nil || rel.Spec == nil || rel.Status == nil {
		return nil
	}
	if strings.TrimSpace(rel.Status.ResolvedVersion) == "" {
		return nil
	}
	required := requiredNodesFromStatus(rel.Status.Nodes, nodes)
	return &DesiredServiceTarget{
		Service:                   strings.TrimSpace(rel.Spec.Component),
		PublisherID:               rel.Spec.PublisherID,
		DesiredVersion:            rel.Status.ResolvedVersion,
		DesiredBuildID:            rel.Status.ResolvedBuildID,
		DesiredEntrypointChecksum: normalizeEntrypointChecksum(rel.Status.ResolvedEntrypointChecksum),
		DesiredPackageDigest:      rel.Status.ResolvedArtifactDigest,
		// COMMAND-kind components have no systemd unit; the verifier itself
		// derives this distinction from the proof's effective-state fields.
		// We keep RuntimeNeeded true here and let absent-unit data degrade
		// the verdict to runtime_identity_unproven rather than mis-classify.
		RuntimeNeeded: true,
		RequiredNodes: required,
		ApplyTime:     lastTransitionFromStatus(rel.Status.LastTransitionUnixMs),
	}
}

// enrichTargetsWithEntrypointChecksum fills DesiredEntrypointChecksum on
// every target by looking up the artifact manifest in the repository.
// One RPC per unique cache key; when DesiredBuildID is present we key by
// build_id, otherwise we key by (publisher, service, version).
//
// Best-effort: a manifest fetch failure leaves the target's
// DesiredEntrypointChecksum empty, which the verifier handles cleanly
// (it degrades to runtime_identity_unproven instead of asserting drift
// against a hole — diagnostic-honesty rule).
func (c *Collector) enrichTargetsWithEntrypointChecksum(ctx context.Context, snap *Snapshot) {
	if c.repoClient == nil || len(snap.DesiredServiceTargets) == 0 {
		return
	}
	cache := newEntrypointCache()
	fetchCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()
	if c.clusterID != "" {
		md := metadata.Pairs("cluster_id", c.clusterID)
		fetchCtx = metadata.NewOutgoingContext(fetchCtx, md)
	}

	for _, dst := range snap.DesiredServiceTargets {
		if dst == nil {
			continue
		}
		info := cache.lookupOrFetch(fetchCtx, c, dst)
		if info.checksum != "" {
			dst.DesiredEntrypointChecksum = info.checksum
		}
		// WrapsUpstreamBinary is sticky on the target — once we
		// recognise a wrapper at any point in the lookup, the verifier
		// should treat it as such. A later cache hit cannot demote it.
		if info.wrapsUpstream {
			dst.WrapsUpstreamBinary = true
		}
	}
}

// manifestInfo bundles the two manifest facts we care about so the cache
// can memoize both with one RPC.
type manifestInfo struct {
	checksum      string
	wrapsUpstream bool
}

// entrypointCache memoizes manifest lookups by build_id so we only pay
// one RPC per unique release across the whole desired set.
type entrypointCache struct {
	mu sync.Mutex
	m  map[string]manifestInfo // cache key → manifest facts
}

func newEntrypointCache() *entrypointCache {
	return &entrypointCache{m: make(map[string]manifestInfo)}
}

// manifestEntrypointsAreNoopOnly returns true when the manifest declares a
// single entrypoint and that entrypoint is the well-known no-op sentinel
// (bin/noop) used by wrapper packages like keepalived and scylladb. The
// sentinel signals that the package ships no real binary — the cluster
// runs an OS-installed binary instead — and the verifier must therefore
// not enforce desired-vs-installed or installed-vs-running binary hashes
// for this package.
func manifestEntrypointsAreNoopOnly(entrypoints []string) bool {
	if len(entrypoints) == 0 {
		return false
	}
	for _, e := range entrypoints {
		if strings.TrimSpace(e) != "bin/noop" {
			return false
		}
	}
	return true
}

func (e *entrypointCache) lookupOrFetch(ctx context.Context, c *Collector, dst *DesiredServiceTarget) manifestInfo {
	bid := strings.TrimSpace(dst.DesiredBuildID)
	key := entrypointCacheKey(dst)
	e.mu.Lock()
	cs, hit := e.m[key]
	e.mu.Unlock()
	if hit {
		return cs
	}

	req := &repopb.GetArtifactManifestRequest{
		Ref: &repopb.ArtifactRef{
			PublisherId: strings.TrimSpace(dst.PublisherID),
			Name:        strings.TrimSpace(dst.Service),
			Version:     strings.TrimSpace(dst.DesiredVersion),
		},
	}
	resp, err := c.repoClient.GetArtifactManifest(ctx, req)
	got := manifestInfo{}
	if err == nil && resp != nil && resp.GetManifest() != nil {
		m := resp.GetManifest()
		got.checksum = normalizeEntrypointChecksum(m.GetEntrypointChecksum())
		got.wrapsUpstream = manifestEntrypointsAreNoopOnly(m.GetEntrypoints())
		// Belt-and-suspenders: verify the manifest's build_id matches
		// the desired build_id we asked for. Mismatch means the
		// resolver and the repository disagree about which build is
		// installable — log it loudly and refuse the value (better to
		// degrade to unproven than to apply the wrong checksum).
		if shouldRejectManifestForBuildMismatch(bid, m.GetBuildId()) {
			log.Printf("verification: repo returned build_id=%s for %s but desired=%s — refusing checksum",
				strings.TrimSpace(m.GetBuildId()), dst.Service, bid)
			got.checksum = ""
			// We still trust the wrapsUpstream signal — it doesn't
			// depend on which build is installable; the package is or
			// isn't a wrapper regardless.
		}
	} else if err != nil {
		log.Printf("verification: GetArtifactManifest(%s) failed: %v", dst.Service, err)
	}

	e.mu.Lock()
	e.m[key] = got
	e.mu.Unlock()
	return got
}

func entrypointCacheKey(dst *DesiredServiceTarget) string {
	if dst == nil {
		return ""
	}
	if bid := strings.TrimSpace(dst.DesiredBuildID); bid != "" {
		return "build:" + bid
	}
	return "svc:" + strings.TrimSpace(dst.PublisherID) + "/" +
		strings.TrimSpace(dst.Service) + "/" + strings.TrimSpace(dst.DesiredVersion)
}

func shouldRejectManifestForBuildMismatch(desiredBuildID, manifestBuildID string) bool {
	db := strings.TrimSpace(desiredBuildID)
	mb := strings.TrimSpace(manifestBuildID)
	if db == "" || mb == "" {
		return false
	}
	return db != mb
}

// normalizeEntrypointChecksum strips the sha256: prefix and lower-cases
// so comparisons in the verifier always succeed when the bytes match.
func normalizeEntrypointChecksum(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	return strings.TrimPrefix(h, "sha256:")
}

// requiredNodesFromStatus extracts the node-id list from the per-node
// status array on a release. Falls back to all known nodes when the
// release has no per-node entries yet (pre-resolve / synthetic release).
func requiredNodesFromStatus(statusNodes []*cluster_controllerpb.NodeReleaseStatus, allNodes []*cluster_controllerpb.NodeRecord) []string {
	if len(statusNodes) > 0 {
		out := make([]string, 0, len(statusNodes))
		for _, n := range statusNodes {
			if id := strings.TrimSpace(n.NodeID); id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	out := make([]string, 0, len(allNodes))
	for _, n := range allNodes {
		if id := strings.TrimSpace(n.GetNodeId()); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func lastTransitionFromStatus(unixMs int64) time.Time {
	if unixMs <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(unixMs)
}

// runVerification is the Phase 9 orchestrator. For every desired target
// it locates the matching per-node proof, calls verifier.VerifyTarget,
// aggregates the result, and persists the per-(node, service) verdict
// under the brief's canonical etcd prefix.
//
// Errors here are logged + folded into snap.DataErrors but never abort
// the rest of the sweep — the doctor's other invariants stay actionable
// even if the verification step is partial.
func (c *Collector) runVerification(ctx context.Context, snap *Snapshot) {
	if len(snap.DesiredServiceTargets) == 0 {
		// Nothing to verify yet (pre-bootstrap, or all releases still
		// WAITING/PENDING). Don't synthesize a result.
		return
	}

	now := time.Now()
	var verdicts []verifier.Verdict

	// Track which (nodeID, service) pairs have been scheduled so the
	// catch-up pass below doesn't duplicate them.
	scheduled := make(map[string]bool)

	for _, dst := range snap.DesiredServiceTargets {
		for _, nodeID := range dst.RequiredNodes {
			scheduled[nodeID+"/"+dst.Service] = true
			installInfo := resolvePerNodeInstallInfo(ctx, nodeID, dst)
			tgt := verifier.Target{
				Service:                   dst.Service,
				NodeID:                    nodeID,
				DesiredVersion:            dst.DesiredVersion,
				DesiredBuildID:            dst.DesiredBuildID,
				DesiredEntrypointChecksum: dst.DesiredEntrypointChecksum,
				DesiredPackageDigest:      dst.DesiredPackageDigest,
				RuntimeNeeded:             dst.RuntimeNeeded,
				ApplyTime:                 installInfo.applyTime,
				ApplyTimeSource:           installInfo.applyTimeSource,
				IsFirstInstall:            installInfo.isFirstInstall,
				WrapsUpstreamBinary:       dst.WrapsUpstreamBinary,
			}
			ev := verifier.Evidence{
				Proof: findProofForService(snap.RuntimeProofs[nodeID], dst.Service),
				// RenderedUnit is intentionally empty: the controller does
				// not yet expose its rendered units through an RPC the
				// doctor can scrape. Phase 5b's effective-config drift
				// sub-check is skipped when RenderedUnit is empty; it
				// remains live for callers that DO have the rendered text.
			}
			verdicts = append(verdicts, verifier.VerifyTarget(tgt, ev, now))
		}
	}

	// Targeted sweep requests: the controller writes these when it detects a
	// persistent runtime_identity_unproven finding past the Day-0 grace window.
	// We inject them now so the pair is verified in this sweep cycle without
	// waiting for the next scheduled pass.
	for _, req := range sweepRequestedPairs(ctx) {
		if scheduled[req.NodeID+"/"+req.Service] {
			continue // already in the normal sweep set; no need to duplicate
		}
		dst, ok := snap.DesiredServiceTargets[req.Service]
		if !ok || dst == nil {
			continue
		}
		scheduled[req.NodeID+"/"+req.Service] = true
		installInfo := resolvePerNodeInstallInfo(ctx, req.NodeID, dst)
		tgt := verifier.Target{
			Service:                   dst.Service,
			NodeID:                    req.NodeID,
			DesiredVersion:            dst.DesiredVersion,
			DesiredBuildID:            dst.DesiredBuildID,
			DesiredEntrypointChecksum: dst.DesiredEntrypointChecksum,
			DesiredPackageDigest:      dst.DesiredPackageDigest,
			RuntimeNeeded:             dst.RuntimeNeeded,
			ApplyTime:                 installInfo.applyTime,
			ApplyTimeSource:           installInfo.applyTimeSource,
			IsFirstInstall:            installInfo.isFirstInstall,
			WrapsUpstreamBinary:       dst.WrapsUpstreamBinary,
		}
		ev := verifier.Evidence{
			Proof: findProofForService(snap.RuntimeProofs[req.NodeID], dst.Service),
		}
		verdicts = append(verdicts, verifier.VerifyTarget(tgt, ev, now))
	}

	// Catch-up pass: verify any (node, service) pair where the service is
	// actually installed on the node but the node is missing from the
	// ServiceRelease's status node list. This happens when a node joins
	// after the release was already AVAILABLE — the status captures only
	// the founding cohort, so RequiredNodes above never includes the new
	// node. Without this pass, the new node gets a permanent FAIL from
	// handlers_health ("no verifier verdict yet") even though the service
	// is installed and running correctly.
	for nodeID, kinds := range snap.NodePackageKinds {
		for svcName := range kinds {
			if scheduled[nodeID+"/"+svcName] {
				continue
			}
			dst, ok := snap.DesiredServiceTargets[svcName]
			if !ok || dst == nil {
				continue
			}
			installInfo := resolvePerNodeInstallInfo(ctx, nodeID, dst)
			tgt := verifier.Target{
				Service:                   dst.Service,
				NodeID:                    nodeID,
				DesiredVersion:            dst.DesiredVersion,
				DesiredBuildID:            dst.DesiredBuildID,
				DesiredEntrypointChecksum: dst.DesiredEntrypointChecksum,
				DesiredPackageDigest:      dst.DesiredPackageDigest,
				RuntimeNeeded:             dst.RuntimeNeeded,
				ApplyTime:                 installInfo.applyTime,
				ApplyTimeSource:           installInfo.applyTimeSource,
				IsFirstInstall:            installInfo.isFirstInstall,
				WrapsUpstreamBinary:       dst.WrapsUpstreamBinary,
			}
			ev := verifier.Evidence{
				Proof: findProofForService(snap.RuntimeProofs[nodeID], dst.Service),
			}
			verdicts = append(verdicts, verifier.VerifyTarget(tgt, ev, now))
		}
	}

	r := verifier.AggregateResult(verdicts, nil, nil, now)
	snap.VerifierResult = &r
	snap.addSource("verifier.AggregateResult")

	c.persistVerificationResults(ctx, snap, &r)
}

// perNodeInstallInfo bundles the per-node install facts resolved from the
// InstalledPackage record for one (nodeID, service) pair.
type perNodeInstallInfo struct {
	applyTime       time.Time
	applyTimeSource string
	isFirstInstall  bool
}

// resolvePerNodeInstallInfo resolves install facts for one (nodeID, service)
// pair with a SINGLE etcd lookup (trying SERVICE kind first, then INFRASTRUCTURE).
// It returns both the correct per-node ApplyTime (from InstalledPackage.InstalledUnix)
// and the IsFirstInstall signal — replacing the old isFirstInstall function that
// could not set ApplyTime.
//
// Using InstalledPackage.InstalledUnix as ApplyTime is the architecture fix for the
// node-join false-positive: ServiceRelease.Status.LastTransitionUnixMs bumps for
// ALL nodes when any node is added, causing process_start_time < new_ApplyTime for
// existing healthy nodes. The per-node InstalledUnix is stable — it only changes
// when that specific node installs a new version.
//
// Fallback chain:
//  1. InstalledPackage.InstalledUnix → "installed_package.installed_unix"
//  2. InstalledPackage.UpdatedUnix  → "installed_package.updated_unix_fallback"
//  3. dst.ApplyTime (release-level) → "release.last_transition_fallback"
//  4. Nothing available             → "unknown"
//
// On lookup failure we default IsFirstInstall=false (treat as upgrade) — that
// keeps the strict default: missing evidence does not turn a real upgrade-bug
// into a benign info row.
func resolvePerNodeInstallInfo(ctx context.Context, nodeID string, dst *DesiredServiceTarget) perNodeInstallInfo {
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE"} {
		pkg, err := installed_state.GetInstalledPackage(ctx, nodeID, kind, dst.Service)
		if err != nil || pkg == nil {
			continue
		}
		installedUnix := pkg.GetInstalledUnix()
		updatedUnix := pkg.GetUpdatedUnix()

		var applyTime time.Time
		var source string
		if installedUnix > 0 {
			applyTime = time.Unix(installedUnix, 0)
			source = "installed_package.installed_unix"
		} else if updatedUnix > 0 {
			applyTime = time.Unix(updatedUnix, 0)
			source = "installed_package.updated_unix_fallback"
		}
		if applyTime.IsZero() {
			continue
		}

		// IsFirstInstall: installed and updated within 60s = same operation (Day-0 install).
		// A fresh install records installedUnix ≈ updatedUnix; any subsequent re-apply
		// bumps updatedUnix while leaving installedUnix alone.
		delta := updatedUnix - installedUnix
		if delta < 0 {
			delta = -delta
		}
		return perNodeInstallInfo{
			applyTime:       applyTime,
			applyTimeSource: source,
			isFirstInstall:  installedUnix > 0 && delta <= 60,
		}
	}
	// Last resort: release-level transition time. Must be explicit fallback.
	if !dst.ApplyTime.IsZero() {
		return perNodeInstallInfo{
			applyTime:       dst.ApplyTime,
			applyTimeSource: "release.last_transition_fallback",
			isFirstInstall:  false, // conservative default — missing installed-state doesn't make old PIDs benign
		}
	}
	return perNodeInstallInfo{applyTimeSource: "unknown"}
}

// findProofForService scans a per-node proofs slice for the matching
// service. Names are canonicalised on both sides so a publisher prefix
// in the desired service name doesn't drop the match. Returns nil when
// no proof is present.
func findProofForService(proofs []*node_agentpb.ServiceRuntimeProof, service string) *node_agentpb.ServiceRuntimeProof {
	want := canonForLookup(service)
	for _, p := range proofs {
		if p == nil {
			continue
		}
		if canonForLookup(p.GetServiceName()) == want {
			return p
		}
		if canonForLookup(p.GetServiceId()) == want {
			return p
		}
	}
	return nil
}

func canonForLookup(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// persistVerificationResults writes one etcd key per per-target verdict
// at /globular/verification/runtime/<node>/<service>. Best-effort; an
// etcd write failure for one target doesn't block the others. Each
// value is the JSON-encoded Verdict for cross-process consumption
// (cluster_controller, ad-hoc CLI, future verifier daemon).
func (c *Collector) persistVerificationResults(ctx context.Context, snap *Snapshot, r *verifier.Result) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		snap.addError("etcd", "GetEtcdClient(verification.write)", err)
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	for _, v := range r.Verdicts {
		key := verifier.EtcdKeyForVerification(v.Target.NodeID, v.Target.Service)
		payload, err := json.Marshal(v)
		if err != nil {
			log.Printf("verification: marshal verdict for %s/%s: %v",
				v.Target.NodeID, v.Target.Service, err)
			continue
		}
		if _, err := cli.Put(writeCtx, key, string(payload)); err != nil {
			snap.addError("etcd", "Put("+key+")", err)
		}
	}
}
