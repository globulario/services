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
	"google.golang.org/grpc/metadata"
)

// fetchDesiredServiceTargets reads ServiceRelease and
// InfrastructureRelease objects via the cluster_controller's typed
// ListServiceReleasesJson + ListInfrastructureReleasesJson RPCs and
// folds them into snap.DesiredServiceTargets keyed by canonical
// service name.
//
// History: prior to v1.2.188 this function scanned
// /globular/resources/{ServiceRelease,InfrastructureRelease}/* in
// etcd directly. The cluster_controller owns those prefixes, so
// cluster_doctor reading raw etcd violated
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
// The controller's handler reads via the same srv.resources typed
// store; xDS-equivalent narrow projections (GetDesiredState) drop
// fields verification needs (Status.Nodes, ResolvedBuildID,
// RequiredNodes), so the JSON-on-wire pattern from ListServices
// is reused.
//
// Best-effort: per-call errors degrade the verification step
// without failing the whole sweep — matches the prior etcd-error
// contract.
func (c *Collector) fetchDesiredServiceTargets(ctx context.Context, snap *Snapshot) {
	if c.controllerClient == nil {
		snap.addError("cluster_controller", "controller client unavailable for verification targets", nil)
		return
	}

	loadCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	// ── ServiceRelease ──
	if srResp, err := c.controllerClient.ListServiceReleasesJson(loadCtx, &cluster_controllerpb.ListServiceReleasesJsonRequest{}); err != nil {
		snap.addError("cluster_controller", "ListServiceReleasesJson", err)
	} else {
		for _, raw := range srResp.GetReleasesJson() {
			var rel cluster_controllerpb.ServiceRelease
			if jerr := json.Unmarshal([]byte(raw), &rel); jerr != nil {
				continue
			}
			tgt := serviceReleaseToTarget(&rel, snap.Nodes)
			if tgt != nil {
				snap.DesiredServiceTargets[tgt.Service] = tgt
			}
		}
		snap.addSource("cluster_controller.ListServiceReleasesJson")
	}

	// ── InfrastructureRelease ──
	if irResp, err := c.controllerClient.ListInfrastructureReleasesJson(loadCtx, &cluster_controllerpb.ListInfrastructureReleasesJsonRequest{}); err != nil {
		snap.addError("cluster_controller", "ListInfrastructureReleasesJson", err)
	} else {
		for _, raw := range irResp.GetReleasesJson() {
			var rel cluster_controllerpb.InfrastructureRelease
			if jerr := json.Unmarshal([]byte(raw), &rel); jerr != nil {
				continue
			}
			tgt := infraReleaseToTarget(&rel, snap.Nodes)
			if tgt != nil {
				snap.DesiredServiceTargets[tgt.Service] = tgt
			}
		}
		snap.addSource("cluster_controller.ListInfrastructureReleasesJson")
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
			if collectorPackageIsCommand(name, kind) {
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

// collectorPackageIsCommand classifies CLI-only packages that have no
// long-running runtime identity to prove. The static list is authoritative for
// legacy/day-0 installs that were historically recorded under INFRASTRUCTURE;
// newer installs can rely on kind=COMMAND directly.
func collectorPackageIsCommand(name, kind string) bool {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "rclone", "restic", "mc", "sctool", "etcdctl", "ffmpeg",
		"globular-cli", "cli", "sha256sum", "yt-dlp", "claude":
		return true
	}
	return strings.EqualFold(strings.TrimSpace(kind), "COMMAND")
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

// manifestEntrypointsAreNoopOnly returns true when the manifest declares no
// Globular-managed entrypoint binary. Two encodings exist in the wild:
//   - the older no-op sentinel "bin/noop"
//   - the newer explicit declaration "none"
//
// Both mean the package is a wrapper: the cluster runs an OS-installed
// binary (or a thin launcher that execs into one), so the verifier must not
// enforce desired-vs-installed or installed-vs-running binary hashes for
// this package.
func manifestEntrypointsAreNoopOnly(entrypoints []string) bool {
	if len(entrypoints) == 0 {
		return false
	}
	for _, e := range entrypoints {
		switch strings.TrimSpace(strings.ToLower(e)) {
		case "bin/noop", "none":
			continue
		default:
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
				OldPidBoundary:            installInfo.oldPidBoundary,
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
		dst := snap.DesiredServiceTargets[req.Service]
		if dst == nil {
			// ServiceRelease is transiently FAILED or missing. Build a minimal
			// target from the installed package so the sweep still produces a
			// verdict. Without this, the sweep request is silently dropped and
			// the service stays UNVERIFIED until the release recovers.
			dst = minimalTargetFromInstalled(ctx, req.NodeID, req.Service)
			if dst == nil {
				continue // truly not installed — nothing to verify
			}
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
			OldPidBoundary:            installInfo.oldPidBoundary,
			WrapsUpstreamBinary:       dst.WrapsUpstreamBinary,
		}
		ev := verifier.Evidence{
			Proof: findProofForService(snap.RuntimeProofs[req.NodeID], dst.Service),
		}
		verdicts = append(verdicts, verifier.VerifyTarget(tgt, ev, now))
	}

	// Catch-up pass: verify any (node, service) pair where the service is
	// actually installed on the node but the node is missing from the
	// ServiceRelease's status node list, or the ServiceRelease is transiently
	// FAILED. This happens when a node joins after the release was already
	// AVAILABLE (status captures only the founding cohort), or when a resolve
	// failure temporarily clears the desired target. Without this pass, the
	// node gets a permanent FAIL from handlers_health ("no verifier verdict
	// yet") even though the service is installed and running correctly.
	for nodeID, kinds := range snap.NodePackageKinds {
		for svcName := range kinds {
			if scheduled[nodeID+"/"+svcName] {
				continue
			}
			dst := snap.DesiredServiceTargets[svcName]
			if dst == nil {
				// ServiceRelease transiently FAILED — build a minimal target so
				// the verifier still writes a verdict. The claim layer in the
				// health handler is responsible for version comparison; the
				// verifier only needs enough to prove binary identity.
				dst = minimalTargetFromInstalled(ctx, nodeID, svcName)
				if dst == nil {
					continue // not installed → skip
				}
			}
			scheduled[nodeID+"/"+svcName] = true
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
				OldPidBoundary:            installInfo.oldPidBoundary,
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
	// oldPidBoundary is the STABLE apply/restart boundary for the verifier's
	// old_pid_after_upgrade timing check — InstalledUnix (which does not move on
	// a no-op metadata reconcile), falling back to UpdatedUnix only when
	// InstalledUnix is zero. Distinct from applyTime (=max), which stays recent
	// for the Day-0/post-upgrade grace window.
	oldPidBoundary time.Time
}

// resolveInstallTimings computes the verifier timing facts from a package's raw
// (installedUnix, updatedUnix) — the pure core of resolvePerNodeInstallInfo,
// separated so the timing semantics are unit-testable without etcd. Returns
// ok=false when neither timestamp is set (caller falls through to the next kind
// or the release-level fallback).
//
// Two DISTINCT timestamps, by design, because two consumers need opposite things:
//   - applyTime = max(installedUnix, updatedUnix): deliberately RECENT so the
//     Day-0/post-upgrade grace window (isDay0UnprovenGrace) suppresses false
//     runtime_identity_unproven while a restarted process's proof is pending.
//   - oldPidBoundary = InstalledUnix (the stable first-install/apply anchor),
//     falling back to UpdatedUnix only when InstalledUnix is zero. This is what
//     the old_pid_after_upgrade timing check compares against. Because a no-op
//     metadata reconcile bumps ONLY UpdatedUnix, using InstalledUnix here means
//     such a bump can no longer fabricate a stale-process finding on a healthy,
//     un-restarted PID.
func resolveInstallTimings(installedUnix, updatedUnix int64) (perNodeInstallInfo, bool) {
	// applyTime (grace window) = most-recent of the two.
	latestUnix := installedUnix
	source := "installed_package.installed_unix"
	if updatedUnix > latestUnix {
		latestUnix = updatedUnix
		source = "installed_package.updated_unix"
	}
	if latestUnix == 0 {
		return perNodeInstallInfo{}, false
	}

	// oldPidBoundary (old_pid timing check) = stable InstalledUnix; UpdatedUnix
	// only as a fallback when InstalledUnix is absent.
	boundaryUnix := installedUnix
	if boundaryUnix == 0 {
		boundaryUnix = updatedUnix
	}
	var oldPidBoundary time.Time
	if boundaryUnix > 0 {
		oldPidBoundary = time.Unix(boundaryUnix, 0)
	}

	// IsFirstInstall: installed and updated within 60s = same operation (Day-0 install).
	// A fresh install records installedUnix ≈ updatedUnix; any subsequent re-apply
	// bumps updatedUnix while leaving installedUnix alone.
	delta := updatedUnix - installedUnix
	if delta < 0 {
		delta = -delta
	}
	return perNodeInstallInfo{
		applyTime:       time.Unix(latestUnix, 0),
		applyTimeSource: source,
		isFirstInstall:  installedUnix > 0 && delta <= 60,
		oldPidBoundary:  oldPidBoundary,
	}, true
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
		if info, ok := resolveInstallTimings(pkg.GetInstalledUnix(), pkg.GetUpdatedUnix()); ok {
			return info
		}
	}
	// Last resort: release-level transition time. Must be explicit fallback.
	// No per-node installed-state exists, so the old_pid boundary can only be
	// the same release-level time — the verifier falls back to ApplyTime anyway
	// when oldPidBoundary is zero, so leaving it equal is explicit, not lossy.
	if !dst.ApplyTime.IsZero() {
		return perNodeInstallInfo{
			applyTime:       dst.ApplyTime,
			applyTimeSource: "release.last_transition_fallback",
			isFirstInstall:  false, // conservative default — missing installed-state doesn't make old PIDs benign
			oldPidBoundary:  dst.ApplyTime,
		}
	}
	return perNodeInstallInfo{applyTimeSource: "unknown"}
}

// minimalTargetFromInstalled builds a DesiredServiceTarget from the etcd
// installed-state record for (nodeID, svcName) when no ServiceRelease target
// is available (e.g. the release is transiently FAILED). The target carries
// no desired version or checksum — the verifier uses it only to prove binary
// identity (installedKnown + runtimeOK), and the claim layer in the health
// handler handles version comparison independently. Returns nil when the
// service is not found in the installed-state for either SERVICE or
// INFRASTRUCTURE kind.
func minimalTargetFromInstalled(ctx context.Context, nodeID, svcName string) *DesiredServiceTarget {
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE"} {
		pkg, err := installed_state.GetInstalledPackage(ctx, nodeID, kind, svcName)
		if err != nil || pkg == nil {
			continue
		}
		return &DesiredServiceTarget{
			Service:       strings.TrimSpace(pkg.GetName()),
			RuntimeNeeded: strings.EqualFold(kind, "SERVICE"),
			RequiredNodes: []string{nodeID},
		}
	}
	return nil
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

	var writeErrors int
	for _, v := range r.Verdicts {
		key := verifier.EtcdKeyForVerification(v.Target.NodeID, v.Target.Service)
		payload, err := json.Marshal(v)
		if err != nil {
			log.Printf("verification: marshal verdict for %s/%s: %v",
				v.Target.NodeID, v.Target.Service, err)
			continue
		}
		if _, err := cli.Put(writeCtx, key, string(payload)); err != nil {
			// Verification writes are a non-critical side-effect — they
			// persist results for cross-process consumers but are NOT inputs
			// to the doctor's own rules. A failed write must not call
			// addError() which would mark the snapshot as DataIncomplete and
			// generate per-key snapshot_source_unavailable findings (70+
			// during transient etcd hiccups).
			// See meta.critical_path_no_non_critical_dependency.
			writeErrors++
		}
	}
	if writeErrors > 0 {
		log.Printf("verification: %d/%d etcd writes failed (non-critical, verdicts still in snapshot)",
			writeErrors, len(r.Verdicts))
	}
}
