package rules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Project S invariant: scylla-manager running but no Scylla cluster
// registered with it.

func mkScyllaInventory(unitState string) *node_agentpb.Inventory {
	return &node_agentpb.Inventory{
		Units: []*node_agentpb.UnitStatus{
			{
				Name:  "globular-scylla-manager.service",
				State: unitState,
			},
		},
	}
}

func mkSnap(invs ...*node_agentpb.Inventory) *collector.Snapshot {
	m := map[string]*node_agentpb.Inventory{}
	for i, inv := range invs {
		m[fmt.Sprintf("node-%d", i)] = inv
	}
	return &collector.Snapshot{Inventories: m}
}

// withTestEndpoint pins the HTTP probe base URL for tests written before
// U.3. After U.3 the rule probes HTTPS first; this helper now also pins
// the HTTPS base to a URL that will fail with connection-refused so the
// pre-U.3 tests exercise the HTTP path. New U.3 tests pin both bases
// explicitly via withTestBases.
func withTestEndpoint(t *testing.T, url string) {
	t.Helper()
	prevHTTPS := scyllaManagerHTTPSBase
	prevHTTP := scyllaManagerHTTPBase
	scyllaManagerHTTPBase = url
	scyllaManagerHTTPSBase = "https://127.0.0.1:1" // unreachable → conn refused → fall back
	t.Cleanup(func() {
		scyllaManagerHTTPSBase = prevHTTPS
		scyllaManagerHTTPBase = prevHTTP
	})
}

// withTestBases pins both bases (HTTPS and HTTP) plus the CA path for
// the U.3 tests that exercise the strict-trust HTTPS path.
func withTestBases(t *testing.T, httpsURL, httpURL, caPath string) {
	t.Helper()
	prevHTTPS := scyllaManagerHTTPSBase
	prevHTTP := scyllaManagerHTTPBase
	prevCA := scyllaManagerCAPath
	scyllaManagerHTTPSBase = httpsURL
	scyllaManagerHTTPBase = httpURL
	scyllaManagerCAPath = caPath
	t.Cleanup(func() {
		scyllaManagerHTTPSBase = prevHTTPS
		scyllaManagerHTTPBase = prevHTTP
		scyllaManagerCAPath = prevCA
	})
}

// 1. The literal Project R bug repro: unit active, /api/v1/clusters returns [] → ERROR
func TestScyllaManagerClusterRegistered_ActiveButEmpty_FiresError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/clusters" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{}) // empty array
	}))
	defer srv.Close()
	withTestEndpoint(t, srv.URL)

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("severity=%v want ERROR", f.Severity)
	}
	if f.InvariantID != "scylla_manager.cluster_registered" {
		t.Errorf("invariant_id=%q", f.InvariantID)
	}
	// Must say this is backup-readiness failure.
	if !strings.Contains(f.Summary, "backup") {
		t.Errorf("summary should mention backup; got: %s", f.Summary)
	}
}

// 2. Healthy state: at least one cluster registered → no finding
func TestScyllaManagerClusterRegistered_ActiveWithCluster_Silent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "abc-123", "name": "globular-internal", "host": "10.0.0.63"},
		})
	}))
	defer srv.Close()
	withTestEndpoint(t, srv.URL)

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("registered cluster must not fire; got %d findings", len(findings))
	}
}

// 3. Unit inactive → rule does not fire (different rule territory)
func TestScyllaManagerClusterRegistered_Inactive_Silent(t *testing.T) {
	// Endpoint should not even be probed in this case; set it to an
	// invalid URL to prove no network call is made.
	withTestEndpoint(t, "http://127.0.0.1:1") // unreachable

	for _, state := range []string{"inactive", "failed", "deactivating"} {
		snap := mkSnap(mkScyllaInventory(state))
		findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
		if len(findings) != 0 {
			t.Errorf("state=%q must not fire; got %d findings", state, len(findings))
		}
	}
}

// 4. HTTP probe failure → inconclusive, no finding (avoid false positives
// during transient network issues; the daemon health probe covers that).
func TestScyllaManagerClusterRegistered_ProbeFails_Silent(t *testing.T) {
	// Server returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	withTestEndpoint(t, srv.URL)

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("HTTP 500 must be inconclusive (silent); got %d", len(findings))
	}
}

// 5. No inventory at all → rule does not fire
func TestScyllaManagerClusterRegistered_NoInventory_Silent(t *testing.T) {
	withTestEndpoint(t, "http://127.0.0.1:1")

	for _, snap := range []*collector.Snapshot{
		nil,
		{}, // zero inventories
		mkSnap(),
	} {
		findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
		if len(findings) != 0 {
			t.Errorf("empty snapshot must not fire; got %d", len(findings))
		}
	}
}

// 6. Multiple nodes, one active running scylla-manager: probe runs, empty
// response fires. Defensive coverage of the "any-node" wording.
func TestScyllaManagerClusterRegistered_MultiNode_AnyActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{}) // empty
	}))
	defer srv.Close()
	withTestEndpoint(t, srv.URL)

	snap := mkSnap(
		mkScyllaInventory("inactive"), // node 1: not running
		mkScyllaInventory("active"),   // node 2: running
	)
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Errorf("any-node-active should trigger probe; got %d findings", len(findings))
	}
}

// 7. Remediation steps point at the package-shipped script (Project S
// enforcement path) and the manual sctool fallback.
func TestScyllaManagerClusterRegistered_RemediationMentionsScript(t *testing.T) {
	f := newScyllaManagerUnregisteredFinding("http", "")
	if len(f.Remediation) < 2 {
		t.Fatalf("expected 2 remediation steps, got %d", len(f.Remediation))
	}
	if !strings.Contains(f.Remediation[0].GetDescription(), "scylla-manager-register-cluster") {
		t.Errorf("first remediation should mention the script; got: %s",
			f.Remediation[0].GetDescription())
	}
	if !strings.Contains(f.Remediation[1].GetDescription(), "sctool cluster add") {
		t.Errorf("second remediation should mention sctool; got: %s",
			f.Remediation[1].GetDescription())
	}
}

