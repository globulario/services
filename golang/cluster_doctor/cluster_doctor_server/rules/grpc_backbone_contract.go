// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=grpc_backbone_contract_health_rule
// @awareness risk=high
package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// grpcBackboneContract surfaces violations of the shared gRPC backbone policy
// from direct collector observations (snapshot DataErrors).
//
// It detects:
//  1) cluster_id propagation failures (metadata contract drift)
//  2) call-depth guard trips (probable circular RPC loops)
//  3) public probe admission regressions (health/auth endpoints unexpectedly denied)
type grpcBackboneContract struct{}

func (grpcBackboneContract) ID() string       { return "grpc.backbone.contract" }
func (grpcBackboneContract) Category() string { return "control_plane" }
func (grpcBackboneContract) Scope() string    { return "cluster" }

func (g grpcBackboneContract) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if len(snap.DataErrors) == 0 {
		return nil
	}
	var findings []Finding
	for _, de := range snap.DataErrors {
		if de.Err == nil {
			continue
		}
		msg := strings.ToLower(de.Err.Error())
		rpc := strings.TrimSpace(de.RPC)

		// Cluster boundary metadata contract.
		if strings.Contains(msg, "cluster_id required after cluster initialization") ||
			strings.Contains(msg, "cluster_id validation failed") ||
			strings.Contains(msg, "cluster_id mismatch") {
			findings = append(findings, Finding{
				FindingID:       FindingID("grpc.backbone.cluster_id_propagation", de.Service, rpc),
				InvariantID:     "grpc.backbone.contract",
				Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:        g.Category(),
				EntityRef:       de.Service + "/" + rpc,
				Summary:         fmt.Sprintf("gRPC metadata contract violated: %s/%s failed cluster_id enforcement (%v)", de.Service, rpc, de.Err),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("collector", rpc, map[string]string{
						"service":   de.Service,
						"rpc":       rpc,
						"error":     de.Err.Error(),
						"contract":  "cluster_id required after initialization",
						"invariant": "grpc.backbone.contract",
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Use shared client dial/context path so cluster_id metadata is injected (globular_client or dialGRPC wrappers).", ""),
					step(2, "Verify interceptor allowlist and cluster-id enforcement logic for the target RPC.", ""),
				},
			})
			continue
		}

		// Circular loop guard.
		if strings.Contains(msg, "call depth") && strings.Contains(msg, "exceeds maximum") {
			findings = append(findings, Finding{
				FindingID:       FindingID("grpc.backbone.call_depth_guard", de.Service, rpc),
				InvariantID:     "grpc.backbone.contract",
				Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:        g.Category(),
				EntityRef:       de.Service + "/" + rpc,
				Summary:         fmt.Sprintf("gRPC call-depth guard tripped on %s/%s (probable circular service call): %v", de.Service, rpc, de.Err),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("collector", rpc, map[string]string{
						"service":   de.Service,
						"rpc":       rpc,
						"error":     de.Err.Error(),
						"contract":  "x-call-depth bounded",
						"invariant": "grpc.backbone.contract",
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Trace service call chain to remove circular dependency edges.", ""),
					step(2, "Confirm caller uses bounded retry/reconnect behavior and does not recursively call upstream on failure paths.", ""),
				},
			})
			continue
		}

		// Public probes/login must not be blocked by authz drift.
		if isPublicProbeRPC(rpc) && (strings.Contains(msg, "unauthenticated") || strings.Contains(msg, "permission denied")) {
			findings = append(findings, Finding{
				FindingID:       FindingID("grpc.backbone.public_probe_admission", de.Service, rpc),
				InvariantID:     "grpc.backbone.contract",
				Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:        g.Category(),
				EntityRef:       de.Service + "/" + rpc,
				Summary:         fmt.Sprintf("Public gRPC probe/admission regression: %s/%s denied (%v)", de.Service, rpc, de.Err),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("collector", rpc, map[string]string{
						"service":   de.Service,
						"rpc":       rpc,
						"error":     de.Err.Error(),
						"contract":  "health/login/reflection probes remain reachable",
						"invariant": "grpc.backbone.contract",
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Check interceptor unauthenticated allowlist for health/login/reflection RPCs.", ""),
					step(2, "Confirm bootstrap/auth boundary changes did not tighten public probe paths unintentionally.", ""),
				},
			})
		}
	}
	return findings
}

func isPublicProbeRPC(rpc string) bool {
	r := strings.ToLower(strings.TrimSpace(rpc))
	return strings.Contains(r, "/grpc.health.v1.health/check") ||
		strings.Contains(r, "/grpc.health.v1.health/watch") ||
		strings.Contains(r, "/grpc.reflection") ||
		strings.Contains(r, "/authentication.authenticationservice/authenticate")
}

