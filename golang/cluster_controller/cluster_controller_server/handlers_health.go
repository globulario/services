// @awareness namespace=globular.platform
// @awareness component=cluster_controller.health
// @awareness file_role=runtime_health_rpc_handlers_for_controller
// @awareness implements=globular.platform:intent.health.requires_fresh_evidence
// @awareness implements=globular.platform:intent.platform_status.is_contract_not_vibe
// @awareness risk=medium
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/verifier"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *server) GetClusterHealth(ctx context.Context, req *cluster_controllerpb.GetClusterHealthRequest) (*cluster_controllerpb.GetClusterHealthResponse, error) {
	srv.lock("cluster-health")
	defer srv.unlock()

	resp := &cluster_controllerpb.GetClusterHealthResponse{
		TotalNodes: int32(len(srv.state.Nodes)),
	}

	now := time.Now()
	healthyThreshold := 2 * time.Minute // Node is healthy if seen within this time

	for _, node := range srv.state.Nodes {
		nodeHealth := &cluster_controllerpb.NodeHealthStatus{
			NodeId:    node.NodeID,
			Hostname:  node.Identity.Hostname,
			LastError: node.LastError,
			LastSeen:  timestamppb.New(node.LastSeen),
		}

		// Determine node health status
		timeSinceSeen := now.Sub(node.LastSeen)
		infraRuntimeOK, infraReason := bootstrapRequiredInfraRuntimeConverged(node, now, srv.state.MinioPoolNodes)
		isHealthy := (node.Status == "healthy" || node.Status == "ready" || node.Status == "converging")
		switch {
		case isHealthy && timeSinceSeen < healthyThreshold && infraRuntimeOK:
			nodeHealth.Status = "healthy"
			resp.HealthyNodes++
		case isHealthy && timeSinceSeen < healthyThreshold && !infraRuntimeOK:
			nodeHealth.Status = "unhealthy"
			nodeHealth.FailedChecks = 1
			nodeHealth.LastError = infraReason
			resp.UnhealthyNodes++
		case node.Status == "unhealthy" || node.Status == "degraded" || node.LastError != "":
			nodeHealth.Status = "unhealthy"
			nodeHealth.FailedChecks = 1
			if node.LastError != "" {
				nodeHealth.LastError = node.LastError
			}
			resp.UnhealthyNodes++
		case timeSinceSeen >= healthyThreshold:
			nodeHealth.Status = "unknown"
			nodeHealth.LastError = fmt.Sprintf("not seen for %v", timeSinceSeen.Round(time.Second))
			resp.UnknownNodes++
		default:
			nodeHealth.Status = "unknown"
			resp.UnknownNodes++
		}

		resp.NodeHealth = append(resp.NodeHealth, nodeHealth)
	}

	// Determine overall cluster status
	switch {
	case resp.TotalNodes == 0:
		resp.Status = "unhealthy"
	case resp.UnhealthyNodes == 0 && resp.UnknownNodes == 0:
		resp.Status = "healthy"
	case resp.HealthyNodes > 0:
		resp.Status = "degraded"
	default:
		resp.Status = "unhealthy"
	}

	return resp, nil
}

func (srv *server) GetClusterHealthV1(ctx context.Context, _ *cluster_controllerpb.GetClusterHealthV1Request) (*cluster_controllerpb.GetClusterHealthV1Response, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if srv.kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "kv unavailable")
	}
	desiredNetObj, err := srv.loadDesiredNetwork(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load desired network: %v", err)
	}
	specHash := ""
	if desiredNetObj != nil && desiredNetObj.Spec != nil {
		hash, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{
			Domain:           desiredNetObj.Spec.GetClusterDomain(),
			Protocol:         desiredNetObj.Spec.GetProtocol(),
			PortHttp:         desiredNetObj.Spec.GetPortHttp(),
			PortHttps:        desiredNetObj.Spec.GetPortHttps(),
			AlternateDomains: append([]string(nil), desiredNetObj.Spec.GetAlternateDomains()...),
			AcmeEnabled:      desiredNetObj.Spec.GetAcmeEnabled(),
			AdminEmail:       desiredNetObj.Spec.GetAdminEmail(),
		})
		specHash = hash
	}
	desiredCanon, _, err := srv.loadDesiredServices(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load desired services: %v", err)
	}
	// Keep a SERVICE-only copy for the privileged-apply check below.
	// Infrastructure packages are managed by bootstrap, not the convergence
	// loop, so they must not trigger PLAN_AWAITING_PRIVILEGED_APPLY.
	serviceOnlyDesired := make(map[string]string, len(desiredCanon))
	for k, v := range desiredCanon {
		serviceOnlyDesired[k] = v
	}
	// Merge InfrastructureRelease entries so infrastructure daemons
	// (etcd, minio, prometheus, etc.) appear in the hash computation
	// alongside gRPC services.
	if srv.resources != nil {
		if items, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
			for _, obj := range items {
				if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
					canon := canonicalServiceName(rel.Spec.Component)
					if canon == "" && rel.Meta != nil {
						canon = canonicalServiceName(rel.Meta.Name)
					}
					if canon != "" {
						if _, exists := desiredCanon[canon]; !exists {
							desiredCanon[canon] = rel.Spec.Version
						}
					}
				}
			}
		}
	}
	srv.lock("health:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, n := range srv.state.Nodes {
		nodes = append(nodes, n)
	}
	srv.unlock()

	var nodeHealths []*cluster_controllerpb.NodeHealth

	for _, node := range nodes {
		if node == nil {
			continue
		}
		appliedNet, _ := srv.getNodeAppliedHash(ctx, node.NodeID)
		filtered := filterVersionsForNode(desiredCanon, node)
		desiredSvcHash := stableServiceDesiredHash(filtered)
		appliedSvcHash, _ := srv.getNodeAppliedServiceHash(ctx, node.NodeID)
		canPriv := false
		if node.Capabilities != nil {
			canPriv = node.Capabilities.CanApplyPrivileged
		}

		// Stamp the applied service hash when all desired services are
		// already installed at the correct version but the hash was never
		// written (e.g. services installed externally via bootstrap/CLI).
		svcOnlyFiltered := filterVersionsForNode(serviceOnlyDesired, node)
		if desiredSvcHash != "" {
			hasMissing := false
			for svc, desiredVer := range svcOnlyFiltered {
				installedVer := ""
				for k, v := range node.InstalledVersions {
					parts := strings.SplitN(k, "/", 2)
					candidate := k
					if len(parts) == 2 {
						candidate = parts[1]
					}
					if canonicalServiceName(candidate) == canonicalServiceName(svc) {
						installedVer = v
						break
					}
				}
				if installedVer != desiredVer {
					hasMissing = true
					break
				}
			}
			if !hasMissing && appliedSvcHash != desiredSvcHash && len(svcOnlyFiltered) > 0 && len(node.InstalledVersions) > 0 {
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, desiredSvcHash); err != nil {
					log.Printf("health: stamp applied service hash for %s: %v (in-memory state NOT updated)", node.NodeID, err)
					// Do not update appliedSvcHash — the etcd write failed, so
					// the durable state does not reflect convergence. Updating
					// the in-memory variable would cause this read RPC to report
					// a hash that was never durably committed.
				} else {
					log.Printf("health: external install detected for node %s — all %d services converged, stamped applied hash", node.NodeID, len(svcOnlyFiltered))
					appliedSvcHash = desiredSvcHash
				}
			}
		}

		nodeHealths = append(nodeHealths, &cluster_controllerpb.NodeHealth{
			NodeId:              node.NodeID,
			DesiredNetworkHash:  specHash,
			AppliedNetworkHash:  appliedNet,
			DesiredServicesHash: desiredSvcHash,
			AppliedServicesHash: appliedSvcHash,
			LastError:           "",
			CanApplyPrivileged:  canPriv,
			InstalledVersions:   node.InstalledVersions,
		})
	}

	// LAW 9: Compute service summaries using pure projection (Desired vs Installed only).
	// No workflow state, no runtime health, no cached counters.
	projections := srv.ComputeClusterProjection(ctx)
	var summaries []*cluster_controllerpb.ServiceSummary
	for _, p := range projections {
		summaries = append(summaries, &cluster_controllerpb.ServiceSummary{
			ServiceName:    p.ServiceName,
			DesiredVersion: p.DesiredVersion,
			NodesAtDesired: int32(p.NodesAtDesired),
			NodesTotal:     int32(p.NodesTotal),
			Kind:           p.Kind,
		})
	}

	return &cluster_controllerpb.GetClusterHealthV1Response{
		Nodes:    nodeHealths,
		Services: summaries,
	}, nil
}

// shortBuildID returns the first 8 characters of a build_id UUID for display.
func shortBuildID(bid string) string {
	if len(bid) <= 8 {
		return bid
	}
	return bid[:8]
}

// Phase 3 (Diagnostic Honesty Refactor) — versionHealthVerdict captures the
// claim-vs-proof breakdown for a single service's version check.
//
//   ProofStatus values:
//     "verified"   — desired claim == installed proof == running proof
//     "claim_only" — only desired-vs-installed claim was checked; no runtime proof consumed
//     "unverified" — proof unavailable / partial; consumer raises service.runtime_identity_unproven
//     "mismatch"   — claim or proof disagrees; finding_id names the specific failure
//
// The Ok bit follows the brief's directive: claim-only is NOT OK. A version
// match without independent runtime proof is degraded, not healthy. Operators
// who need the legacy semantics during transition can set the env var
// GLOBULAR_HEALTH_LEGACY_CLAIM_OK=1 (see legacyClaimOkOverride).
type versionHealthVerdict struct {
	Ok          bool
	Reason      string
	ProofStatus string
	FindingID   string
	// ClaimOK records whether the claim-level check passes regardless of
	// proof state. Operator UIs that key off the legacy bool can read this
	// when the strict Ok bit is unsuitable for their context.
	ClaimOK bool
}

// legacyClaimOkOverride lets operators temporarily restore the pre-Phase-3
// behaviour where a passing claim check produced Ok=true. Default is OFF —
// the brief is explicit that "claim ok=true" alone is forbidden. The escape
// hatch exists so a fleet doesn't go entirely red the instant this change
// ships; it should be removed once Phase 9 (verifier) is live.
func legacyClaimOkOverride() bool {
	v := strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_LEGACY_CLAIM_OK"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// decideVersionVerdict is the Phase 3 entry point that reconciles claim inputs
// against the runtime proof produced by the Phase 9 verifier (Sweep results
// land at /globular/verification/runtime/<node>/<service> as a JSON-encoded
// verifier.Verdict; the caller pre-loads them and passes the matching one in).
//
// Cases:
//   - Claim disagreement → Ok=false, ProofStatus="mismatch", FindingID names the failure.
//   - Claim match + verifier verdict says verified → Ok=true, ProofStatus="verified".
//   - Claim match + verifier verdict says installed_verified → Ok=true,
//     ProofStatus="installed_verified" (binary on disk matches desired; runtime
//     probe agreed or wasn't applicable).
//   - Claim match + verifier verdict says mismatch → Ok=false, ProofStatus="mismatch",
//     FindingID names which proof drifted.
//   - Claim match + verifier verdict unknown / inventory_claim / absent → Ok=false
//     (degraded), ProofStatus="claim_only", FindingID="service.runtime_identity_unproven".
//     Legacy override flips Ok to ClaimOK.
//
// A nil proof is treated as "verifier hasn't run yet" — same as ProofUnknown.
func decideVersionVerdict(desiredVer, desiredBID, installedVer, installedBID string, hasInstalled bool, proof *verifier.Verdict) versionHealthVerdict {
	// ── Claim-level reconciliation (legacy logic) ──────────────────────
	claimOK := false
	claimReason := ""
	if desiredBID != "" && installedBID != "" {
		if desiredBID == installedBID {
			claimOK = true
		} else if installedVer == desiredVer {
			claimOK = true
			claimReason = fmt.Sprintf("build drift: %s installed %s, desired %s",
				desiredVer, shortBuildID(installedBID), shortBuildID(desiredBID))
		} else {
			claimReason = fmt.Sprintf("installed %s, desired %s", installedVer, desiredVer)
		}
	} else if !hasInstalled {
		claimReason = fmt.Sprintf("not installed (desired %s)", desiredVer)
	} else if installedVer != desiredVer {
		claimReason = fmt.Sprintf("installed %s, desired %s", installedVer, desiredVer)
	} else {
		claimOK = true
	}

	// ── Claim disagreement → critical mismatch, regardless of proof ────
	if !claimOK {
		return versionHealthVerdict{
			Ok:          false,
			Reason:      claimReason,
			ProofStatus: "mismatch",
			FindingID:   "service.running_version_mismatch",
			ClaimOK:     false,
		}
	}

	// ── Claim agrees + verifier verdict available → reconcile ─────────
	if proof != nil {
		switch proof.ProofStatus {
		case verifier.ProofRuntimeVerified:
			// Strongest signal: installed bytes match desired AND running
			// process exposes the matching binary hash + version. Health OK.
			return versionHealthVerdict{
				Ok:          true,
				Reason:      "",
				ProofStatus: "verified",
				FindingID:   "",
				ClaimOK:     true,
			}
		case verifier.ProofInstalledVerified:
			// Installed bytes match desired; runtime probe agreed or wasn't
			// applicable (e.g. COMMAND-kind component, no /version endpoint).
			// Healthy — operators see "installed_verified" so the partial
			// nature of the proof is visible.
			return versionHealthVerdict{
				Ok:          true,
				Reason:      "",
				ProofStatus: "installed_verified",
				FindingID:   "",
				ClaimOK:     true,
			}
		case verifier.ProofMismatch:
			// Verifier found drift even though the claim layer agrees —
			// usually new binary on disk with old PID, or installed bytes
			// don't match the desired entrypoint_checksum. Surface the
			// first critical/high finding ID so operators see the precise
			// failure mode rather than the catch-all.
			fid := pickFindingID(proof.Findings, "service.running_binary_hash_mismatch")
			reason := proof.Reason
			if reason == "" {
				reason = "verifier reports drift between claim and runtime proof"
			}
			return versionHealthVerdict{
				Ok:          false,
				Reason:      reason,
				ProofStatus: "mismatch",
				FindingID:   fid,
				ClaimOK:     true,
			}
		}
		// ProofUnknown / ProofInventoryClaim fall through to the
		// unverified-default branch below so operators still see the
		// claim line plus the verifier's degraded reason. Exception:
		// when the verifier emitted runtime_identity_unproven at INFO
		// severity, it's signalling Day-0 first-install grace (proof
		// will land on the next sweep). Treat as OK with a "pending"
		// ProofStatus so the UI doesn't flicker red on a clean install.
		if isDay0UnprovenGraceVerdict(proof) {
			return versionHealthVerdict{
				Ok:          true,
				Reason:      "",
				ProofStatus: "claim_only_day0_grace",
				FindingID:   "",
				ClaimOK:     true,
			}
		}
	}

	// ── Claim agrees + no runtime proof consumed → degraded ───────────
	// Reason carries the claim verdict + the unproven note so operators
	// see exactly why the service is degraded.
	//
	// "no verifier verdict yet" covers two cases that look identical at
	// this layer:
	//   - cluster_doctor sweep hasn't completed since the desired release
	//     resolved (Day-0 cold start; first sweep populates verdicts).
	//   - The verdict was written but ProofStatus is Unknown / InventoryClaim
	//     (per-target evidence was partial — e.g. node-agent's
	//     GetServiceRuntimeProof returned Unimplemented, or the
	//     installed-state lookup raced a write).
	// In either case the operator-visible action is the same: wait for
	// the next doctor sweep, or run `globular cluster doctor --force-fresh`
	// to trigger one. Older message wording implied the verifier itself
	// wasn't implemented; it is.
	reason := "[claim:OK"
	if claimReason != "" {
		reason += " " + claimReason
	}
	reason += "] proof:UNVERIFIED — no verifier verdict yet for this (node, service); finding: service.runtime_identity_unproven"

	if legacyClaimOkOverride() {
		// Escape hatch: restore pre-Phase-3 behaviour during transition.
		return versionHealthVerdict{
			Ok:          true,
			Reason:      claimReason, // mirror legacy text exactly when override active
			ProofStatus: "claim_only",
			FindingID:   "service.runtime_identity_unproven",
			ClaimOK:     true,
		}
	}

	return versionHealthVerdict{
		Ok:          false,
		Reason:      reason,
		ProofStatus: "claim_only",
		FindingID:   "service.runtime_identity_unproven",
		ClaimOK:     true,
	}
}

// isDay0UnprovenGraceVerdict reports whether the verifier's verdict
// indicates a benign, transient situation that should not surface as a
// health-check FAIL. The qualifying case is:
//
//  1. runtime_identity_unproven at INFO — verifier is within its Day-0
//     first-install grace window; proof will land on the next sweep.
//
// With the per-node ApplyTime fix (resolvePerNodeInstallInfo in verification.go),
// bootstrap_ordering_skew no longer fires spuriously on node-join — each node's
// ApplyTime comes from its own InstalledPackage.InstalledUnix, not the
// release-level LastTransitionUnixMs that bumps for all nodes on join.
//
// nil-safe.
func isDay0UnprovenGraceVerdict(v *verifier.Verdict) bool {
	if v == nil {
		return false
	}
	// Only the unproven cases qualify — any other proof_status means
	// the verifier had enough evidence to make a non-grace verdict.
	if v.ProofStatus != verifier.ProofUnknown && v.ProofStatus != verifier.ProofInventoryClaim {
		return false
	}
	for _, f := range v.Findings {
		if f.ID == verifier.FindingRuntimeIdentityUnproven &&
			strings.EqualFold(strings.TrimSpace(f.Severity), verifier.SeverityInfo) {
			return true
		}
	}
	return false
}

// pickFindingID returns the first critical/high finding id from a verifier
// verdict, falling back to a default when none are present. Severity ordering
// matches verifier.SeverityCritical > SeverityHigh > SeverityDegraded.
func pickFindingID(findings []verifier.Finding, fallback string) string {
	// First pass: critical.
	for _, f := range findings {
		if f.Severity == verifier.SeverityCritical && f.ID != "" {
			return f.ID
		}
	}
	// Second pass: high.
	for _, f := range findings {
		if f.Severity == verifier.SeverityHigh && f.ID != "" {
			return f.ID
		}
	}
	// Third pass: anything with an ID.
	for _, f := range findings {
		if f.ID != "" {
			return f.ID
		}
	}
	return fallback
}

// versionCheckDecision is the legacy (bool, string) shim kept for any caller
// that hasn't migrated to decideVersionVerdict. Internally delegates to the
// verdict path so behaviour stays consistent.
func versionCheckDecision(desiredVer, desiredBID, installedVer, installedBID string, hasInstalled bool) (bool, string) {
	v := decideVersionVerdict(desiredVer, desiredBID, installedVer, installedBID, hasInstalled, nil)
	return v.Ok, v.Reason
}

// decideVersionVerdictWithInstallTime extends decideVersionVerdict with a
// fresh-install signal so the controller's UI doesn't red-flag a service
// for the gap between (a) ServiceRelease resolution and (b) the next
// verifier sweep writing the first verdict.
//
// The verifier already downgrades runtime_identity_unproven to INFO
// during its own Day-0 window (isDay0UnprovenGrace), and the controller
// honours that via isDay0UnprovenGraceVerdict. But when the verifier
// hasn't run AT ALL yet for a target, no verdict exists in etcd and
// loadVerifierVerdicts returns no entry — so `proof` is nil and the
// strict path returns FAIL with service.runtime_identity_unproven. On a
// fresh install that gap is purely a doctor-sweep-cadence artefact, not
// a real degradation; the operator sees a flickering red UI for ~1-2
// minutes per service.
//
// This wrapper closes the gap: when proof is nil, claim is OK, and the
// installed package was registered within Day0UnprovenGraceWindow, we
// synthesise the same Day-0 verdict the verifier would have emitted
// (ProofUnknown + runtime_identity_unproven at INFO). The existing
// isDay0UnprovenGraceVerdict branch in decideVersionVerdict then maps
// that to claim_only_day0_grace (Ok=true).
//
// installedUnix=0 disables the synthesis (caller has no signal); the
// behaviour falls through to the existing strict path.
//
// installedAtTrusted=false signals that the caller's installed-state
// lookup could not be observed (e.g. transient etcd outage). When the
// proof is also missing AND no install time is known, the strict path
// would FAIL with runtime_identity_unproven for every service — a
// cluster-wide red badge during a brief etcd flap. Instead, when we
// can't tell whether the service is mid-grace, we behave as if it
// MIGHT be (synthesise the same Day-0 verdict the verifier would have)
// and reason-tag it so an operator can tell observability gap from
// real grace. Next health-detail tick will read again and resolve to
// the actual state. This is the same pattern round-4's
// loadVerifierVerdicts/loadInstalledUnixForNode trusted-bool refactor
// drove top-down: distinguish "I don't know" from "I know it's bad."
func decideVersionVerdictWithInstallTime(
	desiredVer, desiredBID, installedVer, installedBID string,
	hasInstalled bool, proof *verifier.Verdict, installedUnix int64,
	installedAtTrusted bool,
) versionHealthVerdict {
	if proof == nil && installedUnix > 0 {
		if time.Since(time.Unix(installedUnix, 0)) < verifier.Day0UnprovenGraceWindow {
			proof = &verifier.Verdict{
				ProofStatus: verifier.ProofUnknown,
				Findings: []verifier.Finding{{
					ID:       verifier.FindingRuntimeIdentityUnproven,
					Severity: verifier.SeverityInfo,
				}},
				Reason: "no runtime proof yet — within Day-0 first-install grace window (controller-synthesised; verifier sweep has not produced a verdict)",
			}
		}
	} else if proof == nil && installedUnix == 0 && !installedAtTrusted {
		proof = &verifier.Verdict{
			ProofStatus: verifier.ProofUnknown,
			Findings: []verifier.Finding{{
				ID:       verifier.FindingRuntimeIdentityUnproven,
				Severity: verifier.SeverityInfo,
			}},
			Reason: "no runtime proof yet — installed-state read failed; cannot determine Day-0 grace; preserved as INFO until next health-detail tick",
		}
	}
	return decideVersionVerdict(desiredVer, desiredBID, installedVer, installedBID, hasInstalled, proof)
}

// loadInstalledUnixForNode reads every installed package for a node in one
// prefix-Get and returns a map keyed by canonical service name to the
// package's installedUnix timestamp, plus a `trusted` bool that
// distinguishes "the etcd read succeeded" from "we could not consult
// installed-state at all".
//
// Previously this returned `{}` on any error "by design": the
// fresh-install grace is advisory, so an etcd hiccup must not regress
// the rest of the health response. The unintended effect: during a
// transient etcd flap a service that WAS within Day-0 grace was
// reported as expired-grace (proof missing AND no install timestamp)
// and runtime_identity_unproven fired immediately, breaking the grace
// contract pinned by feedback_day0_unproven_grace. The cause was the
// classic meta.authority_must_express_uncertainty: "I have no data" and
// "I couldn't fetch the data" returned the same shape.
//
// New contract: trusted=true means the prefix Get completed (the map
// is the authoritative view, absences included); trusted=false means
// the etcd read failed and absences are NOT authoritative — the caller
// preserves grace by skipping the proof-expired downgrade until the
// next health detail tick can read installed-state.
//
// Mirrors the round-4 loadVerifierVerdicts pattern exactly.
func (srv *server) loadInstalledUnixForNode(ctx context.Context, nodeID string) (map[string]int64, bool) {
	out := map[string]int64{}
	if strings.TrimSpace(nodeID) == "" {
		// No node ID — caller knows this isn't an observable runtime;
		// treat as trusted absence so behavior matches test fixtures
		// and pre-bootstrap startup (same convention as loadVerifierVerdicts).
		return out, true
	}
	pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, "")
	if err != nil {
		return out, false
	}
	for _, p := range pkgs {
		if p == nil {
			continue
		}
		name := strings.TrimSpace(p.GetName())
		if name == "" {
			continue
		}
		// Use max(installedUnix, updatedUnix) so upgrades also get the
		// Day-0 grace window: the service restarts on upgrade and needs
		// ~1-2 min before the proof RPC succeeds. Using only installedUnix
		// (original first-install time) means upgrades never get grace.
		ts := p.GetInstalledUnix()
		if u := p.GetUpdatedUnix(); u > ts {
			ts = u
		}
		if ts == 0 {
			continue
		}
		key := canonicalServiceName(name)
		// On a fresh install both SERVICE and INFRASTRUCTURE may carry
		// the same canonical name; keep the earliest install timestamp
		// so the grace window measures from the first install attempt.
		if existing, ok := out[key]; !ok || ts < existing {
			out[key] = ts
		}
	}
	return out, true
}

// ingressIsDisabled returns true when /globular/ingress/v1/spec
// carries mode="disabled" or explicit_disabled=true. Conservative on
// failure: if the spec cannot be read, is missing, or is malformed,
// returns false so the existing fail-open behaviour is preserved
// (the caller MUST NOT gate ERROR-severity rules on a "disabled"
// determination derived from a read failure). Mirrors the
// cluster_doctor helper.
//
// Routes through the typed srv.loadIngressSpec helper in
// ingress_spec_guard.go rather than reading the etcd key directly.
// Same principle the four-layer authority invariant enforces
// across services: even inside the owner, truth flows through a
// typed boundary.
func (srv *server) ingressIsDisabled(ctx context.Context) bool {
	spec, _, err := srv.loadIngressSpec(ctx)
	if err != nil || spec == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(spec.Mode)), "disabled") || spec.ExplicitDisabled
}

// loadVerifierVerdicts reads all verifier verdicts for a given node from etcd
// in one prefix Get and returns them keyed by canonical service name, plus
// a `trusted` bool that distinguishes "the etcd read succeeded" from "we
// could not consult the verifier store at all".
//
// Previously this returned an empty map on any error "by design", with the
// comment that verifier proofs are "advisory". Downstream then treated
// "service absent from verdict map" as "service unverified" and silently
// fell back to claim-only — `service was verified OK` and `we could not
// observe the verifier` had identical observable behaviour, which is the
// exact silence-into-lies pattern meta.authority_must_express_uncertainty
// forbids (forbidden.error_absorbed_into_empty_map).
//
// New contract: trusted=true means the prefix Get completed (the map is the
// authoritative view, including absences); trusted=false means the etcd
// read failed and absences in the map are NOT authoritative — the caller
// logs the observability gap so the diagnostic is preserved.
//
// Source layout: /globular/verification/runtime/<node_id>/<service_name>
// (see verifier.EtcdKeyForVerification).
func (srv *server) loadVerifierVerdicts(ctx context.Context, nodeID string) (map[string]*verifier.Verdict, bool) {
	out := map[string]*verifier.Verdict{}
	if srv.kv == nil || strings.TrimSpace(nodeID) == "" {
		// No etcd client configured at all — caller knows this is not an
		// observable runtime; treat as trusted absence so behavior matches
		// test fixtures and pre-bootstrap startup.
		return out, true
	}
	prefix := "/globular/verification/runtime/" + strings.TrimSpace(nodeID) + "/"
	resp, err := srv.kv.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return out, false
	}
	if resp == nil {
		return out, true
	}
	for _, kv := range resp.Kvs {
		if kv == nil || len(kv.Value) == 0 {
			continue
		}
		v := &verifier.Verdict{}
		if jerr := json.Unmarshal(kv.Value, v); jerr != nil {
			continue
		}
		// Key looks like /globular/verification/runtime/<node>/<service>; the
		// service portion is the trailing path component. Canonicalize for
		// lookup so caller's iteration (which uses canonicalServiceName)
		// matches verifier's trailing component.
		key := string(kv.Key)
		idx := strings.LastIndex(key, "/")
		if idx < 0 || idx == len(key)-1 {
			continue
		}
		svc := key[idx+1:]
		out[canonicalServiceName(svc)] = v
	}
	return out, true
}

func (srv *server) GetNodeHealthDetailV1(ctx context.Context, req *cluster_controllerpb.GetNodeHealthDetailV1Request) (*cluster_controllerpb.GetNodeHealthDetailV1Response, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := req.GetNodeId()

	srv.lock("health-detail:snapshot")
	node := srv.state.Nodes[nodeID]
	srv.unlock()

	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}

	var checks []*cluster_controllerpb.NodeHealthCheck

	// 1. Heartbeat check
	heartbeatAge := time.Since(node.LastSeen)
	heartbeatOK := !node.LastSeen.IsZero() && heartbeatAge < unhealthyThreshold
	hbReason := ""
	hashIsStale := false
	if !heartbeatOK {
		if node.LastSeen.IsZero() {
			hbReason = "never seen"
		} else if heartbeatAge > heartbeatStaleThreshold {
			hbReason = fmt.Sprintf("unreachable — last seen %s ago, applied hash is stale",
				heartbeatAge.Truncate(time.Second))
			hashIsStale = true
		} else {
			hbReason = fmt.Sprintf("last seen %s ago", heartbeatAge.Truncate(time.Second))
		}
	}
	_ = hashIsStale // used by future hash comparison logic
	checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
		Subsystem: "heartbeat",
		Ok:        heartbeatOK,
		Reason:    hbReason,
	})

	// 2. Unit checks — compare required units from plan vs reported unit states
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	// keepalived is ingress-gated. When /globular/ingress/v1/spec.mode is
	// "disabled" or explicit_disabled=true (e.g. Day-0 default before
	// `globular cluster network ...`), keepalived MUST NOT run; flagging it
	// as inactive is a false-positive. Mirrors cluster_doctor's
	// node_units_running gating.
	ingressDisabled := srv.ingressIsDisabled(ctx)
	unitStates := make(map[string]string, len(node.Units))
	for _, u := range node.Units {
		if u.Name != "" {
			unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
		}
	}
	for unit := range required {
		if ingressDisabled && strings.EqualFold(unit, "keepalived.service") {
			continue
		}
		unitOK := false
		reason := ""
		st, found := unitStates[strings.ToLower(unit)]
		if !found {
			reason = "unit not reported by node"
		} else if st != "active" {
			reason = fmt.Sprintf("state is %q", st)
		} else {
			unitOK = true
		}
		checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
			Subsystem: "unit:" + unit,
			Ok:        unitOK,
			Reason:    reason,
		})
	}

	// 3. Inventory check
	checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
		Subsystem: "inventory",
		Ok:        node.InventoryComplete,
		Reason: func() string {
			if !node.InventoryComplete {
				return "inventory scan not yet complete"
			}
			return ""
		}(),
	})

	// 4. Version checks — compare installed vs desired, filtered by node profile.
	// build_id is the authoritative convergence identity. Version strings are
	// used only as a fallback when build_id is not available on either side.
	//
	// If the desired version's artifact is not yet available in the repository
	// (release phase is WAITING or PENDING with a "not found" message), the
	// version check is OK — the service is running fine with what it has.
	// A mismatch is only a failure when the artifact exists and could be installed.
	desiredCanon, desiredFull, _ := srv.loadDesiredServices(ctx)
	filtered := filterVersionsForNode(desiredCanon, node)
	assignedServices := ServicesForProfiles(node.Profiles)

	// Load Phase 9 verifier verdicts for this node in a single prefix-Get.
	// trusted=false means etcd Get failed and the map's absences are not
	// authoritative — log so consumers of the health response can see
	// the verifier observability gap. The downstream version-check still
	// degrades to claim-only as before, but the gap is now diagnosable
	// (meta.authority_must_express_uncertainty).
	verdicts, verdictsTrusted := srv.loadVerifierVerdicts(ctx, nodeID)
	if !verdictsTrusted {
		log.Printf("GetNodeHealthDetailV1: verifier verdicts unavailable for node=%s — falling back to claim-only; results show 'unverified' but may actually be verified", nodeID)
	}

	// Load installed-package timestamps so the version check can apply
	// Day-0 grace when proof is missing but the install is fresh.
	// Without this, the gap between ServiceRelease resolution and the
	// next verifier sweep red-flags services for ~1-2 minutes per
	// service on every fresh install. installedAtTrusted=false means
	// the etcd read failed; downstream MUST treat absences in
	// installedAt as "unknown" rather than "no install" so transient
	// etcd flap doesn't burn the Day-0 grace window.
	installedAt, installedAtTrusted := srv.loadInstalledUnixForNode(ctx, nodeID)
	if !installedAtTrusted {
		log.Printf("GetNodeHealthDetailV1: installed-state read failed for node=%s — Day-0 grace preserved by assuming services are still fresh; next tick will retry", nodeID)
	}

	// Build a set of services whose release is AVAILABLE but artifact was never
	// published. These are at their best possible version — not a health failure.
	acceptedAsIs := make(map[string]bool)
	for svc := range filtered {
		releaseName := fmt.Sprintf("core@globular.io/%s", svc)
		if rel, err := srv.GetServiceRelease(ctx, &cluster_controllerpb.GetServiceReleaseRequest{Name: releaseName}); err == nil && rel != nil {
			if rel.Status.Phase == cluster_controllerpb.ReleasePhaseAvailable &&
				rel.Status.TransitionReason == "artifact_not_published" {
				acceptedAsIs[svc] = true
			}
		}
	}

	for svc, desiredVer := range filtered {
		// Skip services not assigned to this node's profiles.
		// Infrastructure/command packages are always checked; only workloads are filtered.
		if comp, ok := catalogIndex[svc]; ok && comp.Kind == KindWorkload && !assignedServices[svc] {
			continue
		}

		// If the desired artifact was never published, the installed version is correct.
		// This is the one path where claim-only OK is correct semantically — there's
		// no artifact to verify against, so proof_status remains empty.
		if acceptedAsIs[svc] {
			checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
				Subsystem: "version:" + svc,
				Ok:        true,
				Reason:    "",
			})
			continue
		}

		var desiredBID string
		if sdv := desiredFull[svc]; sdv != nil && sdv.Spec != nil {
			desiredBID = sdv.Spec.BuildID
		}
		// Use lookupInstalledVersionFromMap (and its build-id sibling) so we
		// match publisher-prefixed keys like "core@globular.io/sql" against
		// the desired short name "sql". Direct map access misses those and
		// reports "not installed" for services that are actually installed.
		installedVer := lookupInstalledVersionFromMap(node.InstalledVersions, svc)
		hasInstalled := installedVer != ""
		installedBID := lookupInstalledVersionFromMap(node.InstalledBuildIDs, svc)
		canonSvc := canonicalServiceName(svc)
		proof := verdicts[canonSvc]
		installedUnix := installedAt[canonSvc]
		v := decideVersionVerdictWithInstallTime(
			desiredVer, desiredBID, installedVer, installedBID,
			hasInstalled, proof, installedUnix, installedAtTrusted,
		)

		// Self-heal: when the verdict is claim_only (UNVERIFIED) with
		// service.runtime_identity_unproven and we are past the Day-0 grace
		// window, trigger a targeted sweep so the doctor picks this pair up on
		// the next cycle rather than waiting for the full sweep schedule.
		// We do NOT trigger during the grace window — the verifier itself
		// handles Day-0 gracefully and a targeted sweep is unnecessary noise.
		if v.ProofStatus == "claim_only" && v.FindingID == verifier.FindingRuntimeIdentityUnproven {
			pastGrace := installedUnix > 0 &&
				time.Since(time.Unix(installedUnix, 0)) >= verifier.Day0UnprovenGraceWindow
			if pastGrace {
				// Enrich reason with the expected verification key so operators
				// can inspect or manually trigger a doctor sweep.
				expectedKey := verifier.EtcdKeyForVerification(nodeID, svc)
				if v.Reason != "" && !strings.Contains(v.Reason, expectedKey) {
					v.Reason += fmt.Sprintf("; expected_key=%s", expectedKey)
				}
				go requestVerifierSweep(ctx, nodeID, svc, verifier.FindingRuntimeIdentityUnproven)
			}
		}

		checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
			Subsystem:   "version:" + svc,
			Ok:          v.Ok,
			Reason:      v.Reason,
			ProofStatus: v.ProofStatus,
			FindingId:   v.FindingID,
		})
	}

	// Overall status from existing evaluator, overridden to unhealthy if heartbeat fails.
	overallStatus, _ := srv.evaluateNodeStatus(node, node.Units)
	if !heartbeatOK {
		overallStatus = "unhealthy"
	}
	allOK := true
	for _, c := range checks {
		if !c.Ok {
			allOK = false
			break
		}
	}

	canPriv := false
	privReason := ""
	if node.Capabilities != nil {
		canPriv = node.Capabilities.CanApplyPrivileged
		privReason = node.Capabilities.PrivilegeReason
	}

	return &cluster_controllerpb.GetNodeHealthDetailV1Response{
		NodeId:             nodeID,
		OverallStatus:      overallStatus,
		Healthy:            allOK,
		Checks:             checks,
		LastError:          node.LastError,
		CanApplyPrivileged: canPriv,
		InventoryComplete:  node.InventoryComplete,
		LastSeen:           timestamppb.New(node.LastSeen),
		PrivilegeReason:    privReason,
	}, nil
}

func (srv *server) monitorNodeHealth(ctx context.Context) {
	now := time.Now()

	srv.lock("health-monitor:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	srv.unlock()

	var stateDirty bool

	for _, node := range nodes {
		timeSinceSeen := now.Sub(node.LastSeen)

		srv.lock("health-monitor:check")
		currentNode := srv.state.Nodes[node.NodeID]
		if currentNode == nil {
			srv.unlock()
			continue
		}

		previousStatus := currentNode.Status

		// Check if node is unhealthy
		if timeSinceSeen > unhealthyThreshold {
			currentNode.FailedHealthChecks++

			newStatus := "unhealthy"
		if timeSinceSeen > heartbeatStaleThreshold {
			newStatus = "unreachable"
		}
		if currentNode.Status != newStatus {
				currentNode.Status = newStatus
				currentNode.MarkedUnhealthySince = now
				currentNode.LastError = fmt.Sprintf("no contact for %v", timeSinceSeen.Round(time.Second))
				log.Printf("node %s marked %s: %s", node.NodeID, newStatus, currentNode.LastError)
				srv.emitClusterEvent("cluster.health.degraded", map[string]interface{}{
					"severity":       "WARNING",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"reason":         currentNode.LastError,
					"correlation_id": fmt.Sprintf("node:%s", node.NodeID),
				})
				stateDirty = true
			}

			// Attempt recovery if needed
			shouldRecover := currentNode.RecoveryAttempts < maxRecoveryAttempts &&
				(currentNode.LastRecoveryAttempt.IsZero() || now.Sub(currentNode.LastRecoveryAttempt) > recoveryAttemptInterval)

			if shouldRecover && node.AgentEndpoint != "" {
				currentNode.LastRecoveryAttempt = now
				currentNode.RecoveryAttempts++
				stateDirty = true
				log.Printf("attempting recovery for node %s (attempt %d/%d)", node.NodeID, currentNode.RecoveryAttempts, maxRecoveryAttempts)
				srv.unlock()

				// Attempt to reconnect and redispatch plan
				if err := srv.attemptNodeRecovery(ctx, node); err != nil {
					log.Printf("recovery attempt for node %s failed: %v", node.NodeID, err)
					srv.lock("health-monitor:recovery-failed")
					if n := srv.state.Nodes[node.NodeID]; n != nil {
						n.LastError = fmt.Sprintf("recovery failed: %v", err)
					}
					srv.unlock()
				} else {
					log.Printf("recovery attempt for node %s initiated successfully", node.NodeID)
				}
				continue
			}
		} else if (currentNode.Status == "unhealthy" || currentNode.Status == "unreachable") &&
		(previousStatus == "unhealthy" || previousStatus == "unreachable") {
			// Node came back online - reset recovery counters
			currentNode.Status = "healthy"
			currentNode.FailedHealthChecks = 0
			currentNode.RecoveryAttempts = 0
			currentNode.MarkedUnhealthySince = time.Time{}
			currentNode.LastError = ""
			log.Printf("node %s recovered and marked healthy", node.NodeID)
			srv.emitClusterEvent("cluster.health.recovered", map[string]interface{}{
				"severity":       "INFO",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"correlation_id": fmt.Sprintf("node:%s", node.NodeID),
			})
			stateDirty = true
		}
		srv.unlock()
	}

	if stateDirty {
		srv.lock("health-monitor:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("health monitor: persist state: %v", err)
			}
		}()
	}
}

func (srv *server) attemptNodeRecovery(ctx context.Context, node *nodeState) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("no agent endpoint for node %s", node.NodeID)
	}

	// Close any existing connection to force reconnection
	srv.closeAgentClient(node.AgentEndpoint)

	// Get fresh agent client
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to agent: %w", err)
	}

	// Try to get inventory to verify connectivity
	_, err = client.GetInventory(ctx)
	if err != nil {
		return fmt.Errorf("get inventory: %w", err)
	}

	// If we can connect, dispatch the current plan
	plan, planErr := srv.computeNodePlan(node)
	if planErr != nil {
		return fmt.Errorf("compute plan: %w", planErr)
	}
	if plan == nil || (len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0) {
		// No plan needed, just mark as recovered
		return nil
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "recovery: plan queued", 0, false, ""))

	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "recovery: plan failed", 0, true, err.Error()))
		return fmt.Errorf("dispatch plan: %w", err)
	}

	// Phase 4b: store pending rendered config hashes on recovery dispatch.
	// Promoted to RenderedConfigHashes only after agent reports apply success.
	if len(plan.GetRenderedConfig()) > 0 {
		srv.lock("recovery-rendered-config-hashes")
		node.PendingRenderedConfigHashes = HashRenderedConfigs(plan.GetRenderedConfig())
		srv.unlock()
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "recovery: plan dispatched", 25, false, ""))
	return nil
}

func (srv *server) evaluateNodeStatus(node *nodeState, units []unitStatusRecord) (string, string) {
	if node == nil {
		return "degraded", "missing node record"
	}
	// Unreachable: no heartbeat beyond stale threshold.
	if !node.LastSeen.IsZero() && time.Since(node.LastSeen) > heartbeatStaleThreshold {
		return "unreachable", fmt.Sprintf("no heartbeat for %s",
			time.Since(node.LastSeen).Truncate(time.Second))
	}
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	if len(required) == 0 {
		return "ready", ""
	}
	// MinIO topology contract: when a node is explicitly held as non_member
	// (pending apply-topology admission), minio and its sidecar are intentionally
	// not running. This is not a health failure — exclude them from the required
	// set so the node can reach "ready" status and allow service releases to
	// converge on the node.
	if node.MinioJoinPhase == MinioJoinNonMember {
		delete(required, "globular-minio.service")
		delete(required, "globular-sidekick.service")
		if len(required) == 0 {
			return "ready", ""
		}
	}
	// keepalived is ingress-gated — see GetNodeHealthDetailV1 for full
	// rationale. Best-effort lookup with a short timeout; failure preserves
	// the existing fail-open behaviour.
	ingressDisabled := false
	if srv.kv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ingressDisabled = srv.ingressIsDisabled(ctx)
		cancel()
	}
	unitStates := make(map[string]string, len(units))
	for _, u := range units {
		if u.Name == "" {
			continue
		}
		unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
	}
	var missing []string
	var notActive []string
	for unit := range required {
		if ingressDisabled && strings.EqualFold(unit, "keepalived.service") {
			continue
		}
		state, ok := unitStates[strings.ToLower(unit)]
		if !ok {
			missing = append(missing, fmt.Sprintf("%s missing", unit))
			continue
		}
		if state != "active" {
			if state == "" {
				state = "unknown"
			}
			notActive = append(notActive, fmt.Sprintf("%s is %s", unit, state))
		}
	}
	if len(missing) > 0 || len(notActive) > 0 {
		reason := strings.Join(append(missing, notActive...), "; ")
		if node.ReportedAt.IsZero() || time.Since(node.ReportedAt) < statusGracePeriod {
			return "converging", reason
		}
		return "degraded", reason
	}
	return "ready", ""
}

func (srv *server) startHealthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	safeGoTracked("health-monitor", healthCheckInterval, func(h *globular_service.SubsystemHandle) {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Controller self-update runs on all instances (followers and leader).
				srv.reconcileControllerSelfUpdate(ctx)
				if !srv.isLeader() {
					h.Tick()
					continue
				}
				srv.monitorNodeHealth(ctx)
				h.Tick()
			}
		}
	})
}

// GetSubsystemHealth returns the health state of all registered background
// subsystems (goroutines) in this controller process.
func (srv *server) GetSubsystemHealth(_ context.Context, _ *cluster_controllerpb.GetControllerSubsystemHealthRequest) (*cluster_controllerpb.GetControllerSubsystemHealthResponse, error) {
	entries := globular_service.SubsystemSnapshot()
	resp := &cluster_controllerpb.GetControllerSubsystemHealthResponse{
		Subsystems: make([]*cluster_controllerpb.ControllerSubsystemHealth, 0, len(entries)),
		Overall:    toControllerSubsystemState(globular_service.SubsystemOverallState()),
	}
	for _, e := range entries {
		sh := &cluster_controllerpb.ControllerSubsystemHealth{
			Name:       e.Name,
			State:      toControllerSubsystemState(e.State),
			LastError:  e.LastError,
			ErrorCount: e.ErrorCount,
			Metadata:   e.Metadata,
		}
		if !e.LastTick.IsZero() {
			sh.LastTick = timestamppb.New(e.LastTick)
		}
		resp.Subsystems = append(resp.Subsystems, sh)
	}
	return resp, nil
}

func toControllerSubsystemState(s globular_service.SubsystemState) cluster_controllerpb.ControllerSubsystemState {
	switch s {
	case globular_service.SubsystemHealthy:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_HEALTHY
	case globular_service.SubsystemDegraded:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_DEGRADED
	case globular_service.SubsystemFailed:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_FAILED
	case globular_service.SubsystemStarting:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_STARTING
	case globular_service.SubsystemStopped:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_STOPPED
	default:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_UNSPECIFIED
	}
}
