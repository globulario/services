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
// Note: DesiredEntrypointChecksum is NOT populated here — the release
// status carries only the package tarball digest. The collector fills
// the binary checksum in enrichTargetsWithEntrypointChecksum by looking
// up the repository manifest.
func serviceReleaseToTarget(rel *cluster_controllerpb.ServiceRelease, nodes []*cluster_controllerpb.NodeRecord) *DesiredServiceTarget {
	if rel == nil || rel.Spec == nil || rel.Status == nil {
		return nil
	}
	if strings.TrimSpace(rel.Status.ResolvedVersion) == "" {
		return nil // not yet resolved; nothing to verify
	}
	required := requiredNodesFromStatus(rel.Status.Nodes, nodes)
	return &DesiredServiceTarget{
		Service:              strings.TrimSpace(rel.Spec.ServiceName),
		PublisherID:          rel.Spec.PublisherID,
		DesiredVersion:       rel.Status.ResolvedVersion,
		DesiredBuildID:       rel.Status.ResolvedBuildID,
		DesiredPackageDigest: rel.Status.ResolvedArtifactDigest, // tarball — kept separate
		RuntimeNeeded:        true,                              // ServiceRelease implies a running systemd unit
		RequiredNodes:        required,
		ApplyTime:            lastTransitionFromStatus(rel.Status.LastTransitionUnixMs),
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
		Service:              strings.TrimSpace(rel.Spec.Component),
		PublisherID:          rel.Spec.PublisherID,
		DesiredVersion:       rel.Status.ResolvedVersion,
		DesiredBuildID:       rel.Status.ResolvedBuildID,
		DesiredPackageDigest: rel.Status.ResolvedArtifactDigest,
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
// One RPC per unique build_id; results cached so a re-resolution of the
// same release doesn't re-fetch.
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

	for _, dst := range snap.DesiredServiceTargets {
		if dst == nil || strings.TrimSpace(dst.DesiredBuildID) == "" {
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
	m  map[string]manifestInfo // build_id → manifest facts
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
	e.mu.Lock()
	cs, hit := e.m[bid]
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
		if rid := strings.TrimSpace(m.GetBuildId()); rid != "" && rid != bid {
			log.Printf("verification: repo returned build_id=%s for %s but desired=%s — refusing checksum",
				rid, dst.Service, bid)
			got.checksum = ""
			// We still trust the wrapsUpstream signal — it doesn't
			// depend on which build is installable; the package is or
			// isn't a wrapper regardless.
		}
	} else if err != nil {
		log.Printf("verification: GetArtifactManifest(%s) failed: %v", dst.Service, err)
	}

	e.mu.Lock()
	e.m[bid] = got
	e.mu.Unlock()
	return got
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

	for _, dst := range snap.DesiredServiceTargets {
		for _, nodeID := range dst.RequiredNodes {
			tgt := verifier.Target{
				Service:                   dst.Service,
				NodeID:                    nodeID,
				DesiredVersion:            dst.DesiredVersion,
				DesiredBuildID:            dst.DesiredBuildID,
				DesiredEntrypointChecksum: dst.DesiredEntrypointChecksum,
				DesiredPackageDigest:      dst.DesiredPackageDigest,
				RuntimeNeeded:             dst.RuntimeNeeded,
				ApplyTime:                 dst.ApplyTime,
				IsFirstInstall:            isFirstInstall(ctx, nodeID, dst),
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

	r := verifier.AggregateResult(verdicts, nil, nil, now)
	snap.VerifierResult = &r
	snap.addSource("verifier.AggregateResult")

	c.persistVerificationResults(ctx, snap, &r)
}

// isFirstInstall returns true when the InstalledPackage for this
// (node, service) was written within ~60 seconds of its updatedUnix —
// i.e. install and update were the same operation, no upgrade has run
// since. The heuristic is loose on purpose: a fresh install records
// installedUnix == updatedUnix; any subsequent re-apply bumps
// updatedUnix while leaving installedUnix alone.
//
// The result drives the verifier's classification of an
// older-than-apply process as bootstrap_ordering_skew (degraded) on
// first install vs old_pid_after_upgrade (critical) on upgrade.
//
// On lookup failure we default to false (treat as upgrade) — that
// keeps the strict default: missing evidence does not turn a real
// upgrade-bug into a benign info row.
func isFirstInstall(ctx context.Context, nodeID string, dst *DesiredServiceTarget) bool {
	if dst == nil {
		return false
	}
	pkg, err := installed_state.GetInstalledPackage(ctx, nodeID, "SERVICE", dst.Service)
	if err != nil || pkg == nil {
		// Try INFRASTRUCTURE kind before giving up — same package may
		// register under either depending on its catalog kind.
		pkg, err = installed_state.GetInstalledPackage(ctx, nodeID, "INFRASTRUCTURE", dst.Service)
		if err != nil || pkg == nil {
			return false
		}
	}
	installedUnix := pkg.GetInstalledUnix()
	updatedUnix := pkg.GetUpdatedUnix()
	if installedUnix == 0 {
		return false
	}
	// Within 60s = same operation. The Day-0 install path writes
	// installed and updated within milliseconds of each other.
	delta := updatedUnix - installedUnix
	if delta < 0 {
		delta = -delta
	}
	return delta <= 60
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
