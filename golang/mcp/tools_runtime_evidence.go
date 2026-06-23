// @awareness namespace=globular.platform
// @awareness component=platform_mcp.tools_runtime_evidence
// @awareness file_role=runtime_evidence_collector_globular_adapter_for_awg_runtime_proof_lane
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness implements=globular.platform:intent.awareness.mcp_tools_use_gateway_client_pool
// @awareness enforces=meta.discovery_produces_candidates_not_facts
// @awareness enforces=meta.storage_is_not_semantic_authority
// @awareness risk=high
package main

// tools_runtime_evidence.go — Phase 2b of the AWG runtime proof lane: the LIVE
// Globular collector. It is the platform adapter that turns Globular's runtime
// surfaces into a normalized AWG runtime-evidence/v1 snapshot, which the
// (out-of-platform) AWG spine then diagnoses, repairs, gates, and — at most —
// proposes governance candidates from.
//
// The whole point of the offline spine (Phases 1·2a·3·4·5·6) was to make THIS
// step narrow and honest: every hard decision (what is fresh, what diagnosis a
// snapshot maps to, what authorizes a repair, what may only become a candidate)
// already lives behind the runtime-evidence/v1 schema boundary in AWG core. This
// collector's only job is to PRODUCE that evidence truthfully:
//
//   - It declares per-lane freshness/owner/observed_at honestly. A source that is
//     unreachable or whose proof is INDETERMINATE is labelled unavailable/unknown
//     — never silently dropped and never upgraded to "fresh". (This mirrors the
//     release-boundary rule in release_boundary/proof.go: "a tool/source failure
//     is evidence, not silence — it can never be upgraded to PROVEN", and the
//     AWG intent evidence.provenance_trust_levels.)
//   - It never DIAGNOSES. It assembles evidence; the verdict is AWG's
//     `cluster-diagnose`. If the collector lies, the spine still fails closed:
//     stale/unknown evidence cannot go green, and nothing here can self-promote
//     past a review-gated candidate. Read-only OBSERVE tier — no mutation.
//     (Honors repair_plan globular.repair.runtime_evidence_stale_or_conflicting:
//     no convergence/PASS claim may rest on stale or conflicting evidence.)
//
// runtime_identity is anchored on the release-boundary evaluator
// (release_boundary/boundarycheck), the authoritative "is the running thing the
// expected thing" proof — richer than a bare installed-package read. PROVEN maps
// to fresh; INDETERMINATE/NOT_APPLICABLE map to unknown; and runtime_identity is
// listed in verdict_inputs.required_lanes so AWG REFUSES to certify convergence
// for a subject whose identity cannot be proven ("unknown must not green").
//
// The pure builder buildRuntimeSnapshot is fixture-tested (no RPCs); the thin
// handler supplies live evidence. Platform specifics live HERE, in the adapter
// — AWG core never learns Globular's RPC names.

import (
	"context"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	release_boundary "github.com/globulario/services/golang/release_boundary"
	"github.com/globulario/services/golang/release_boundary/boundarycheck"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
)

const (
	runtimeEvidenceSchemaVersion = "runtime-evidence/v1"
	runtimeEvidenceAdapterName   = "globular-runtime-evidence-adapter"
	runtimeEvidenceAdapterVer    = "0.2.0" // 0.2: runtime_identity anchored on release-boundary evaluator
)

// ── normalized output: a mirror of AWG's runtime-evidence/v1 schema ──────────
//
// These types intentionally duplicate the shape AWG core validates
// (cmd/awg/cmd_runtime.go:validateRuntimeSnapshot). The collector IS the
// platform adapter, so it must encode the target schema; the cross-boundary
// guarantee is the end-to-end check `awg runtime-snapshot validate <file>`.

type rtSnapshot struct {
	SchemaVersion string            `yaml:"schema_version"`
	Platform      string            `yaml:"platform"`
	ClusterID     string            `yaml:"cluster_id,omitempty"`
	GeneratedAt   string            `yaml:"generated_at"`
	Adapter       rtAdapter         `yaml:"adapter"`
	Subject       rtSubject         `yaml:"subject"`
	Lanes         map[string]rtLane `yaml:"lanes"`
	VerdictInputs rtVerdictInputs   `yaml:"verdict_inputs"`
}

type rtAdapter struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type rtSubject struct {
	Type string `yaml:"type"`
	ID   string `yaml:"id"`
	Node string `yaml:"node,omitempty"`
}

type rtLane struct {
	Status     string                 `yaml:"status"`
	Freshness  string                 `yaml:"freshness"`
	Owner      string                 `yaml:"owner"`
	Source     string                 `yaml:"source"`
	ObservedAt string                 `yaml:"observed_at,omitempty"`
	Facts      map[string]interface{} `yaml:"facts,omitempty"`
	Findings   []rtFinding            `yaml:"findings,omitempty"`
}

type rtFinding struct {
	ID       string `yaml:"id"`
	Severity string `yaml:"severity"`
	Summary  string `yaml:"summary"`
}

type rtVerdictInputs struct {
	RequiredLanes []string `yaml:"required_lanes"`
}

// ── collected evidence: the platform-neutral input to the pure builder ───────

type collectedFinding struct{ ID, Severity, Category, Summary string }

// collectedEvidence is everything the live handler gathered. Each *Reachable
// flag records whether the owning source actually answered — an unreachable
// source becomes an unavailable lane, not a missing one and never a fresh one.
type collectedEvidence struct {
	SubjectType string
	SubjectID   string
	Node        string
	ClusterID   string
	Platform    string
	GeneratedAt string // supplied by the caller; the pure builder is deterministic

	// desired_state — cluster_controller (owner) via GetDesiredState.
	DesiredReachable   bool
	DesiredFound       bool
	DesiredVersion     string
	DesiredBuildNumber int64

	// runtime_identity / observed_state — release-boundary evaluator (authority).
	BoundaryReachable bool
	BoundaryVerdict   string // PROVEN | FAILED | INDETERMINATE | NOT_APPLICABLE
	BoundaryBuildID   string
	DesiredBuildID    string // A1 evidence
	InstalledBuildID  string // A2 evidence
	InstalledPresent  bool   // A2 proven / installed_build_id present
	RunningExeSHA     string // A3 evidence
	Running           bool
	Checksum          string

	// diagnosis / health — cluster_doctor (diagnostic).
	DoctorReachable  bool
	DoctorFindings   []collectedFinding
	DoctorFreshMode  string
	DoctorAgeSeconds int64
	DoctorObservedAt int64
	DoctorSource     string

	// health — cluster_controller health view.
	HealthReachable bool
	NodeConverged   bool
	NodeStatus      string

	// topology — cluster_controller (owner) via ListNodes.
	TopologyReachable bool
	NodeCount         int
	StorageNodeCount  int
}

// identityFreshness is THE honesty hook. PROVEN is a clean proof; FAILED is a
// proven mismatch — both are fresh evidence. INDETERMINATE and NOT_APPLICABLE
// are NOT fresh: identity could not be established and must not be upgraded.
func identityFreshness(reachable bool, verdict string) string {
	if !reachable {
		return "unavailable"
	}
	switch verdict {
	case string(release_boundary.VerdictProven), string(release_boundary.VerdictFailed):
		return "fresh"
	default:
		return "unknown"
	}
}

// reachableFreshness maps an owner source that just answered to "fresh", and an
// unreachable one to "unavailable". Owner truth read live IS fresh by definition
// (the owner authoritatively answered now); the collector does not cache.
func reachableFreshness(reachable bool) string {
	if reachable {
		return "fresh"
	}
	return "unavailable"
}

func reachableStatus(reachable bool) string {
	if reachable {
		return "present"
	}
	return "unavailable"
}

// doctorFreshness derives a lane freshness from the doctor's own freshness block.
// A forced-fresh scan is fresh; a cached snapshot is fresh only while young,
// else stale. Doctor lanes are diagnostic (not required for the convergence
// verdict), so this never gates green — but it is still labelled honestly.
func doctorFreshness(reachable bool, mode string, ageSeconds int64) string {
	if !reachable {
		return "unavailable"
	}
	if strings.Contains(strings.ToUpper(mode), "FRESH") {
		return "fresh"
	}
	if ageSeconds >= 0 && ageSeconds <= 60 {
		return "fresh"
	}
	return "stale"
}

// doctorSeverityToLane maps a Globular doctor severity onto the AWG lane-finding
// severity vocabulary. error/critical become "blocking"/"critical" so the AWG
// diagnoser treats them as blocking; warn/info stay advisory.
func doctorSeverityToLane(sev string) string {
	switch strings.ToLower(sev) {
	case "critical":
		return "critical"
	case "error":
		return "blocking"
	case "warn", "warning":
		return "warning"
	default:
		return "info"
	}
}

// buildRuntimeSnapshot is the PURE normalizer (fixture-tested; no RPCs). It maps
// collected Globular evidence onto runtime-evidence/v1 lanes, declaring freshness
// and authority honestly. It performs NO diagnosis — that is AWG's job.
func buildRuntimeSnapshot(ev collectedEvidence) rtSnapshot {
	observedAt := ev.GeneratedAt
	lanes := map[string]rtLane{}

	// desired_state — owner: cluster_controller.
	{
		facts := map[string]interface{}{}
		if ev.DesiredReachable {
			if ev.DesiredFound {
				facts["desired"] = "present"
				if ev.DesiredBuildID != "" {
					facts["desired_build_id"] = ev.DesiredBuildID
				}
				if ev.DesiredVersion != "" {
					facts["desired_version"] = ev.DesiredVersion
				}
				facts["desired_build_number"] = ev.DesiredBuildNumber
			} else {
				facts["desired"] = "absent"
			}
		}
		lanes["desired_state"] = rtLane{
			Status:     reachableStatus(ev.DesiredReachable),
			Freshness:  reachableFreshness(ev.DesiredReachable),
			Owner:      "cluster_controller",
			Source:     "cluster_get_desired_state",
			ObservedAt: observedAt,
			Facts:      facts,
		}
	}

	// observed_state — owner: node_agent (via the boundary evaluator's installed
	// + runtime evidence). installed=false with running=true is an honest,
	// surfaceable state (a process runs but no proven install record exists).
	{
		facts := map[string]interface{}{}
		if ev.BoundaryReachable {
			facts["installed"] = ev.InstalledPresent
			facts["running"] = ev.Running
			if ev.DesiredVersion != "" {
				facts["installed_version"] = ev.DesiredVersion
			}
		}
		lanes["observed_state"] = rtLane{
			Status:     reachableStatus(ev.BoundaryReachable),
			Freshness:  reachableFreshness(ev.BoundaryReachable),
			Owner:      "node_agent",
			Source:     "release_verify_boundary",
			ObservedAt: observedAt,
			Facts:      facts,
		}
	}

	// runtime_identity — authority: the release-boundary proof. Only PROVEN/FAILED
	// are fresh; INDETERMINATE/NOT_APPLICABLE are unknown, and because this lane is
	// REQUIRED, an unprovable identity blocks any green ("unknown must not green").
	{
		facts := map[string]interface{}{
			"boundary_verdict": ev.BoundaryVerdict,
			"identity_proven":  ev.BoundaryVerdict == string(release_boundary.VerdictProven),
		}
		if ev.InstalledBuildID != "" {
			facts["installed_build_id"] = ev.InstalledBuildID
		}
		if ev.RunningExeSHA != "" {
			facts["running_exe_sha256"] = ev.RunningExeSHA
		}
		if ev.BoundaryBuildID != "" {
			facts["build_id"] = ev.BoundaryBuildID
		}
		if ev.Checksum != "" {
			facts["checksum"] = ev.Checksum
		}
		lanes["runtime_identity"] = rtLane{
			Status:     reachableStatus(ev.BoundaryReachable),
			Freshness:  identityFreshness(ev.BoundaryReachable, ev.BoundaryVerdict),
			Owner:      "node_agent",
			Source:     "release_verify_boundary",
			ObservedAt: observedAt,
			Facts:      facts,
		}
	}

	// diagnosis — diagnostic authority: cluster_doctor. Subject-scoped findings.
	{
		findings := make([]rtFinding, 0, len(ev.DoctorFindings))
		for _, f := range ev.DoctorFindings {
			findings = append(findings, rtFinding{
				ID:       f.ID,
				Severity: doctorSeverityToLane(f.Severity),
				Summary:  f.Summary,
			})
		}
		lanes["diagnosis"] = rtLane{
			Status:     reachableStatus(ev.DoctorReachable),
			Freshness:  doctorFreshness(ev.DoctorReachable, ev.DoctorFreshMode, ev.DoctorAgeSeconds),
			Owner:      "cluster_doctor",
			Source:     "cluster_get_doctor_report",
			ObservedAt: doctorObservedAt(ev),
			Findings:   findings,
		}
	}

	// health — diagnostic: per-node convergence (desired hash == applied hash).
	{
		facts := map[string]interface{}{}
		if ev.HealthReachable {
			facts["converged"] = ev.NodeConverged
			if ev.NodeStatus != "" {
				facts["node_status"] = ev.NodeStatus
			}
		}
		lanes["health"] = rtLane{
			Status:     reachableStatus(ev.HealthReachable),
			Freshness:  reachableFreshness(ev.HealthReachable),
			Owner:      "cluster_doctor",
			Source:     "cluster_get_health",
			ObservedAt: observedAt,
			Facts:      facts,
		}
	}

	// topology — owner: cluster_controller. Membership shape (drives quorum
	// reasoning downstream; the collector reports shape, AWG draws conclusions).
	{
		facts := map[string]interface{}{}
		if ev.TopologyReachable {
			facts["node_count"] = ev.NodeCount
			facts["storage_node_count"] = ev.StorageNodeCount
		}
		lanes["topology"] = rtLane{
			Status:     reachableStatus(ev.TopologyReachable),
			Freshness:  reachableFreshness(ev.TopologyReachable),
			Owner:      "cluster_controller",
			Source:     "cluster_list_nodes",
			ObservedAt: observedAt,
			Facts:      facts,
		}
	}

	return rtSnapshot{
		SchemaVersion: runtimeEvidenceSchemaVersion,
		Platform:      "globular",
		ClusterID:     ev.ClusterID,
		GeneratedAt:   ev.GeneratedAt,
		Adapter:       rtAdapter{Name: runtimeEvidenceAdapterName, Version: runtimeEvidenceAdapterVer},
		Subject:       rtSubject{Type: ev.SubjectType, ID: ev.SubjectID, Node: ev.Node},
		Lanes:         lanes,
		// desired_state + observed_state are the minimum to reach a verdict;
		// runtime_identity is added so an unprovable identity cannot go green.
		VerdictInputs: rtVerdictInputs{RequiredLanes: []string{"desired_state", "observed_state", "runtime_identity"}},
	}
}

func doctorObservedAt(ev collectedEvidence) string {
	if !ev.DoctorReachable || ev.DoctorObservedAt <= 0 {
		return ""
	}
	return time.Unix(ev.DoctorObservedAt, 0).UTC().Format(time.RFC3339)
}

// assertionEvidence indexes a boundary report's assertions by ID for lookup.
func assertionEvidence(r release_boundary.Report) map[string]release_boundary.AssertionReport {
	out := map[string]release_boundary.AssertionReport{}
	for _, a := range r.Assertions {
		out[string(a.ID)] = a
	}
	return out
}

// ── live tool ────────────────────────────────────────────────────────────────

func registerRuntimeEvidenceTools(s *server) {
	s.register(toolDef{
		Name: "runtime_snapshot_collect",
		Description: "Collect a normalized AWG runtime-evidence/v1 snapshot for one subject (service) on one node, " +
			"from live Globular sources (desired state, release-boundary proof, doctor findings, health, topology). " +
			"Read-only. Declares per-lane freshness/owner honestly: an INDETERMINATE release boundary is reported as " +
			"runtime_identity freshness=unknown so the AWG spine cannot certify convergence it cannot prove. " +
			"Feed the returned YAML to `awg cluster-diagnose --runtime-evidence <file>`. Does NOT diagnose or mutate.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject_id":   {Type: "string", Description: "Service identifier to snapshot (e.g. \"event\"). Matched against desired-state ServiceId."},
				"subject_type": {Type: "string", Description: "Subject kind for the snapshot (default \"service\")."},
				"node_id":      {Type: "string", Description: "Node to inspect. Optional when the cluster has exactly one node; required otherwise."},
				"freshness":    {Type: "string", Description: "Doctor snapshot mode: 'cached' (default) or 'fresh' (force a new scan)."},
			},
			Required: []string{"subject_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subjectID := getStr(args, "subject_id")
		if subjectID == "" {
			subjectID = getStr(args, "service_id")
		}
		if subjectID == "" {
			return nil, fmt.Errorf("subject_id is required")
		}
		subjectType := getStr(args, "subject_type")
		if subjectType == "" {
			subjectType = "service"
		}
		nodeID := getStr(args, "node_id")

		ev := collectedEvidence{
			SubjectType: subjectType,
			SubjectID:   subjectID,
			Platform:    "globular",
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		}

		// Topology + node resolution (cluster_controller, owner). ListNodes also
		// lets us default the node when the cluster is single-node.
		if conn, err := s.clients.get(ctx, controllerEndpoint()); err == nil {
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
			resp, lerr := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
			cancel()
			if lerr == nil {
				ev.TopologyReachable = true
				ev.NodeCount = len(resp.GetNodes())
				for _, n := range resp.GetNodes() {
					for _, p := range n.GetProfiles() {
						if p == "storage" {
							ev.StorageNodeCount++
							break
						}
					}
				}
				if nodeID == "" && len(resp.GetNodes()) == 1 {
					nodeID = resp.GetNodes()[0].GetNodeId()
				}
			}
		}
		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required (cluster has %d nodes; cannot default)", ev.NodeCount)
		}
		ev.Node = nodeID

		// Cluster identity (best-effort).
		if conn, err := s.clients.get(ctx, controllerEndpoint()); err == nil {
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
			if info, ierr := client.GetClusterInfo(callCtx, &timestamppb.Timestamp{}); ierr == nil {
				ev.ClusterID = info.GetClusterId()
			}
			cancel()
		}

		// desired_state (cluster_controller, owner).
		if conn, err := s.clients.get(ctx, controllerEndpoint()); err == nil {
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
			state, derr := client.GetDesiredState(callCtx, &emptypb.Empty{})
			cancel()
			if derr == nil {
				ev.DesiredReachable = true
				for _, svc := range state.GetServices() {
					if svc.GetServiceId() == subjectID {
						ev.DesiredFound = true
						ev.DesiredVersion = svc.GetVersion()
						ev.DesiredBuildNumber = svc.GetBuildNumber()
						break
					}
				}
			}
		}

		// runtime_identity + observed_state (release-boundary authority). Run never
		// hard-fails: a collection failure surfaces as INDETERMINATE, which the
		// pure builder maps to freshness=unknown — honest, not silent.
		report, _ := boundarycheck.Run(ctx, s.releaseBoundaryFetchers(ctx, nodeID), subjectID, nodeID, boundarycheck.Options{})
		ev.BoundaryReachable = true
		ev.BoundaryVerdict = string(report.Verdict)
		ev.BoundaryBuildID = report.BuildID
		ev.Checksum = report.Checksum
		as := assertionEvidence(report)
		ev.DesiredBuildID = as[string(release_boundary.AssertionDesiredPublished)].Evidence["desired_build_id"]
		instA := as[string(release_boundary.AssertionInstalledMatches)]
		ev.InstalledBuildID = instA.Evidence["installed_build_id"]
		ev.InstalledPresent = ev.InstalledBuildID != "" || instA.Verdict == release_boundary.VerdictProven
		rtA := as[string(release_boundary.AssertionRuntimeMatches)]
		ev.RunningExeSHA = rtA.Evidence["running_exe_sha256"]
		ev.Running = ev.RunningExeSHA != ""

		// diagnosis (cluster_doctor, diagnostic) — subject-scoped findings.
		if conn, err := s.clients.get(ctx, doctorEndpoint()); err == nil {
			client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
			callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
			dreport, rerr := client.GetClusterReport(callCtx, &cluster_doctorpb.ClusterReportRequest{Freshness: freshnessArg(args)})
			cancel()
			if rerr == nil {
				ev.DoctorReachable = true
				if h := dreport.GetHeader(); h != nil {
					ev.DoctorFreshMode = h.GetFreshnessMode().String()
					ev.DoctorAgeSeconds = h.GetSnapshotAgeSeconds()
					ev.DoctorSource = h.GetSource()
					if h.GetObservedAt() != nil {
						ev.DoctorObservedAt = h.GetObservedAt().GetSeconds()
					}
				}
				for _, f := range dreport.GetFindings() {
					if !findingMentionsSubject(f.GetSummary(), subjectID) {
						continue
					}
					ev.DoctorFindings = append(ev.DoctorFindings, collectedFinding{
						ID:       f.GetFindingId(),
						Severity: severityStr(f.GetSeverity()),
						Category: f.GetCategory(),
						Summary:  f.GetSummary(),
					})
				}
			}
		}

		// health (cluster_controller health view).
		if conn, err := s.clients.get(ctx, controllerEndpoint()); err == nil {
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
			hresp, herr := client.GetClusterHealthV1(callCtx, &cluster_controllerpb.GetClusterHealthV1Request{})
			cancel()
			if herr == nil {
				for _, n := range hresp.GetNodes() {
					if n.GetNodeId() == nodeID {
						ev.HealthReachable = true
						ev.NodeConverged = n.GetDesiredServicesHash() == n.GetAppliedServicesHash() && n.GetLastError() == ""
						if ev.NodeConverged {
							ev.NodeStatus = "healthy"
						} else {
							ev.NodeStatus = "drifted"
						}
						break
					}
				}
			}
		}

		snap := buildRuntimeSnapshot(ev)
		out, merr := yaml.Marshal(snap)
		if merr != nil {
			return nil, fmt.Errorf("marshal snapshot: %w", merr)
		}

		return map[string]interface{}{
			"subject":          subjectType + ":" + subjectID + "@" + nodeID,
			"snapshot_yaml":    string(out),
			"lane_count":       len(snap.Lanes),
			"required_lanes":   snap.VerdictInputs.RequiredLanes,
			"boundary_verdict": ev.BoundaryVerdict,
			"hint":             "feed snapshot_yaml to: awg cluster-diagnose --runtime-evidence <file>",
		}, nil
	})
}

// findingMentionsSubject scopes doctor findings to the subject by a simple,
// conservative name match on the finding summary. Over-inclusion is safer than
// dropping a relevant finding silently; AWG severity-gates what actually blocks.
func findingMentionsSubject(summary, subjectID string) bool {
	if subjectID == "" {
		return false
	}
	return strings.Contains(strings.ToLower(summary), strings.ToLower(subjectID))
}
