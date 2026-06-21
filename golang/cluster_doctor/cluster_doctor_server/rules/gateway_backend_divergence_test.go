package rules

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// The PR-15 fixture: the exact class the ai_memory gateway route bug exposed —
// the gateway answers text/html while the backend is healthy on its direct port.
// The doctor must report "route suspected, backend healthy", NOT "service down".
func TestGatewayBackendDivergence_RouteBrokenBackendHealthy(t *testing.T) {
	snap := &collector.Snapshot{
		GatewayBackendProbes: []collector.GatewayBackendProbe{{
			Service:            "ai_memory.AiMemoryService",
			GatewayEndpoint:    "globular.internal:443",
			BackendEndpoint:    "10.0.0.63:10009",
			GatewayReachable:   false,
			GatewayHTML:        true,
			GatewayContentType: "text/html",
			GatewayErr:         `unexpected content-type "text/html"`,
			BackendChecked:     true,
			BackendReachable:   true,
		}},
	}
	findings := gatewayBackendDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("want INVARIANT_FAIL, got %v", f.InvariantStatus)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("want SEVERITY_ERROR, got %v", f.Severity)
	}
	// It must NOT say the service is down; it must implicate the gateway route.
	low := strings.ToLower(f.Summary)
	if strings.Contains(low, "is down") || strings.Contains(low, "service down") {
		t.Errorf("summary must not call the service down: %q", f.Summary)
	}
	if !strings.Contains(low, "route") || !strings.Contains(low, "backend is healthy") {
		t.Errorf("summary must implicate the route and affirm backend health: %q", f.Summary)
	}
	if len(f.Evidence) == 0 || f.Evidence[0].KeyValues["gateway_content_type"] != "text/html" {
		t.Errorf("evidence must carry gateway_content_type=text/html")
	}
	// Remediation must steer away from permanently bypassing the gateway.
	joined := ""
	for _, r := range f.Remediation {
		joined += " " + strings.ToLower(r.Description)
	}
	if !strings.Contains(joined, "envoy") {
		t.Errorf("remediation should point at the Envoy route/filter-chain: %q", joined)
	}
	if !strings.Contains(joined, "direct backend port") {
		t.Errorf("remediation should warn against permanently using the direct backend port: %q", joined)
	}
}

// Gateway HTML + backend also unreachable → FAIL, but framed as route-broken AND
// backend-not-serving (not a false "healthy backend" claim).
func TestGatewayBackendDivergence_RouteHTMLAndBackendDown(t *testing.T) {
	snap := &collector.Snapshot{
		GatewayBackendProbes: []collector.GatewayBackendProbe{{
			Service:            "ai_memory.AiMemoryService",
			GatewayHTML:        true,
			GatewayContentType: "text/html",
			BackendChecked:     true,
			BackendReachable:   false,
			BackendErr:         "connection refused",
		}},
	}
	findings := gatewayBackendDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 1 || findings[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Fatalf("want 1 FAIL finding, got %+v", findings)
	}
	if strings.Contains(strings.ToLower(findings[0].Summary), "backend is healthy") {
		t.Errorf("must not claim backend healthy when it is unreachable: %q", findings[0].Summary)
	}
}

// Gateway HTML + backend not cross-checked → indeterminate (UNKNOWN/CHECK_ERROR),
// never a confident FAIL.
func TestGatewayBackendDivergence_BackendUnchecked_IsIndeterminate(t *testing.T) {
	snap := &collector.Snapshot{
		GatewayBackendProbes: []collector.GatewayBackendProbe{{
			Service:            "ai_memory.AiMemoryService",
			GatewayHTML:        true,
			GatewayContentType: "text/html",
			BackendChecked:     false,
		}},
	}
	findings := gatewayBackendDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(findings))
	}
	if findings[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("want INVARIANT_UNKNOWN, got %v", findings[0].InvariantStatus)
	}
	if findings[0].CheckError == "" {
		t.Errorf("indeterminate finding must carry a CheckError")
	}
}

// A gateway path that is merely unavailable (NOT an HTML content-type) is
// inconclusive — reflection may legitimately not route through the gateway — so
// a healthy gateway is never falsely reported as broken.
func TestGatewayBackendDivergence_PlainUnavailableIsNotAFalsePositive(t *testing.T) {
	snap := &collector.Snapshot{
		GatewayBackendProbes: []collector.GatewayBackendProbe{{
			Service:          "ai_memory.AiMemoryService",
			GatewayReachable: false,
			GatewayHTML:      false,
			GatewayErr:       "rpc error: code = Unavailable desc = connection error",
			BackendChecked:   true,
			BackendReachable: true,
		}},
	}
	if findings := (gatewayBackendDivergence{}).Evaluate(snap, Config{}); len(findings) != 0 {
		t.Fatalf("plain unavailable must not produce a finding, got %d", len(findings))
	}
}

// Healthy gateway path and empty probe set both produce no findings.
func TestGatewayBackendDivergence_HealthyAndEmpty(t *testing.T) {
	healthy := &collector.Snapshot{
		GatewayBackendProbes: []collector.GatewayBackendProbe{{
			Service:          "ai_memory.AiMemoryService",
			GatewayReachable: true,
			BackendChecked:   true,
			BackendReachable: true,
		}},
	}
	if findings := (gatewayBackendDivergence{}).Evaluate(healthy, Config{}); len(findings) != 0 {
		t.Fatalf("healthy gateway must produce no finding, got %d", len(findings))
	}
	if findings := (gatewayBackendDivergence{}).Evaluate(&collector.Snapshot{}, Config{}); findings != nil {
		t.Fatalf("empty probe set must produce nil, got %d", len(findings))
	}
}
