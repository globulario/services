// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.invariant_registry
// @awareness file_role=degraded_mode_gateway_backend_divergence_rule
// @awareness implements=globular.platform:intent.doctor.findings_are_operator_language
// @awareness risk=medium
package rules

// gatewayBackendDivergence is a degraded-mode rule (PR-15). It turns the
// collector's two-path reachability probes into operator-language findings that
// distinguish a BROKEN GATEWAY ROUTE from a DOWN SERVICE.
//
// The motivating case: the ai_memory gateway route answered "text/html" while
// the ai_memory backend was healthy on its direct port. The wrong diagnosis is
// "ai_memory is down"; the right diagnosis is "ai_memory backend is healthy; the
// gateway route is broken." This rule encodes that distinction.
//
// Honest by construction: a gateway path that is merely unavailable (not an
// HTML/non-gRPC content-type) is inconclusive — reflection does not normally
// route through the gateway — and produces NO finding, so a healthy gateway is
// never falsely reported as broken.

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type gatewayBackendDivergence struct{}

func (gatewayBackendDivergence) ID() string       { return "gateway.backend_divergence" }
func (gatewayBackendDivergence) Category() string { return "control_plane" }
func (gatewayBackendDivergence) Scope() string    { return "service" }

func (g gatewayBackendDivergence) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if len(snap.GatewayBackendProbes) == 0 {
		return nil
	}
	var findings []Finding
	for _, p := range snap.GatewayBackendProbes {
		// Healthy gateway path — nothing to report.
		if p.GatewayReachable {
			continue
		}
		// Only the distinctive HTML/non-gRPC content-type is a trustworthy
		// "gateway route broken" signal. A plain unavailable is inconclusive
		// (reflection may legitimately not route through the gateway), so we
		// emit nothing and never falsely accuse a healthy gateway.
		if !p.GatewayHTML {
			continue
		}

		ev := kvEvidence("cluster_doctor", "gateway_backend_divergence", map[string]string{
			"service":              p.Service,
			"gateway_endpoint":     p.GatewayEndpoint,
			"backend_endpoint":     p.BackendEndpoint,
			"gateway_content_type": p.GatewayContentType,
			"gateway_error":        p.GatewayErr,
			"backend_reachable":    fmt.Sprintf("%t", p.BackendReachable),
			"backend_error":        p.BackendErr,
		})

		switch {
		case p.BackendReachable:
			// THE key case: route broken, backend healthy. NOT service-down.
			findings = append(findings, Finding{
				FindingID:       FindingID(g.ID(), p.Service, "route_suspected_backend_healthy"),
				InvariantID:     g.ID(),
				Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:        g.Category(),
				EntityRef:       p.Service,
				Summary:         fmt.Sprintf("%s gateway route is broken (answers %q) but the backend is healthy on its direct port — the gateway route/filter-chain is suspected, the service is NOT down.", p.Service, p.GatewayContentType),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence:        []*cluster_doctorpb.Evidence{ev},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("Inspect the Envoy route/listener/filter-chain and SNI mapping for %s — a gRPC route has fallen through to a web/HTML handler.", p.Service), ""),
					step(2, "Compare against a service whose gateway route works; check for IP-as-SNI suppressing the gRPC filter chain (same class as the awareness-graph SNI fix).", ""),
					step(3, "Do NOT make this permanent by pointing clients at the direct backend port — that hides the broken route instead of fixing it.", ""),
				},
			})
		case p.BackendChecked:
			// Gateway returns HTML and the direct backend is also unreachable.
			findings = append(findings, Finding{
				FindingID:       FindingID(g.ID(), p.Service, "route_html_backend_unreachable"),
				InvariantID:     g.ID(),
				Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:        g.Category(),
				EntityRef:       p.Service,
				Summary:         fmt.Sprintf("%s gateway route answers %q AND the direct backend port is unreachable — the route is broken and the backend is not serving gRPC.", p.Service, p.GatewayContentType),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
				Evidence:        []*cluster_doctorpb.Evidence{ev},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("Verify the %s backend process/health on its direct port first; a broken gateway route cannot be confirmed while the backend is also down.", p.Service), ""),
					step(2, "Then inspect the Envoy route/filter-chain for the service.", ""),
				},
			})
		default:
			// Gateway returns HTML but we could not cross-check the backend.
			// Strong route evidence, but we must not claim the service is up or
			// down — indeterminate (CHECK_ERROR), not FAIL.
			findings = append(findings, Finding{
				FindingID:       FindingID(g.ID(), p.Service, "route_html_backend_unchecked"),
				InvariantID:     g.ID(),
				Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
				Category:        g.Category(),
				EntityRef:       p.Service,
				Summary:         fmt.Sprintf("%s gateway route answers %q; the direct backend could not be cross-checked, so service health is indeterminate.", p.Service, p.GatewayContentType),
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
				CheckError:      "backend endpoint unavailable to cross-check the gateway route failure",
				Evidence:        []*cluster_doctorpb.Evidence{ev},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("Resolve the %s direct backend endpoint and re-run the doctor to confirm whether the backend is healthy.", p.Service), ""),
				},
			})
		}
	}
	return findings
}
