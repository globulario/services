package rules

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Project U.3 — HTTPS-first probe path in the cluster-doctor invariant
// `scylla_manager.cluster_registered`.
//
// The matrix this file covers:
//
//   U.3-1 HTTPS available + trusted              → use HTTPS, no finding when cluster exists
//   U.3-2 HTTPS unavailable (conn refused)       → fall back to HTTP, scheme=http evidence
//   U.3-3 HTTPS reachable, untrusted cert        → no fallback, WARN finding "TLS trust failure"
//   U.3-4 HTTPS available + empty cluster list   → ERROR finding with scheme=https evidence
//   U.3-5 HTTP-only legacy manager               → still supported (no HTTPS port at all)

// writeCAPEM writes the test TLS server's cert as a PEM CA file. Returns
// the path.
func writeCAPEM(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	cert := srv.Certificate()
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err := os.WriteFile(caPath, pemBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return caPath
}

// writeBogusCA writes a self-signed CA that does NOT trust the test
// scylla-manager server's cert. Returns the path.
//
// NOTE: cannot reuse another httptest.NewTLSServer.Certificate() here —
// httptest reuses a single static localhost cert across all instances, so
// the "bogus" pool would contain the same cert as the target and
// verification would silently succeed. We generate a fresh self-signed
// CA on the fly.
func writeBogusCA(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	caPath := filepath.Join(dir, "bogus-ca.crt")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-bogus-ca"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(caPath, pemBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return caPath
}

// allocateUnboundPort claims and releases a TCP port. The returned
// address has no listener — connections to it will be refused.
func allocateUnboundPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// 1. HTTPS available + trusted + cluster exists → no finding, used HTTPS.
func TestU3_HTTPSAvailableTrusted_NoFindingWhenClusterExists(t *testing.T) {
	httpsHits := 0
	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/clusters" {
			httpsHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "abc", "name": "globular-internal", "host": "10.0.0.63"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer httpsSrv.Close()
	httpHits := 0
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHits++
	}))
	defer httpSrv.Close()
	withTestBases(t, httpsSrv.URL, httpSrv.URL, writeCAPEM(t, httpsSrv))

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("HTTPS-trusted with cluster present must be silent; got %d findings", len(findings))
	}
	if httpsHits == 0 {
		t.Error("HTTPS endpoint was not probed; rule did not prefer HTTPS")
	}
	if httpHits != 0 {
		t.Errorf("HTTP endpoint was probed %d times; rule must NOT fall back when HTTPS works", httpHits)
	}
}

// 2. HTTPS port unbound → fall back to HTTP, no false positive when HTTP
// reports cluster exists.
func TestU3_HTTPSConnectionRefused_FallsBackToHTTP(t *testing.T) {
	httpHits := 0
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/clusters" {
			httpHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "abc", "name": "globular-internal", "host": "10.0.0.63"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer httpSrv.Close()

	deadHTTPS := "https://" + allocateUnboundPort(t)
	// CA doesn't matter here; HTTPS will fail with conn-refused before TLS.
	withTestBases(t, deadHTTPS, httpSrv.URL, "/var/lib/globular/pki/ca.crt")

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("HTTP fallback with cluster present must be silent; got %d findings", len(findings))
	}
	if httpHits == 0 {
		t.Error("HTTP endpoint was not probed; fallback did not happen")
	}
}

// 3. HTTPS reachable + untrusted cert → no fallback, WARN finding with
// TLS trust evidence.
func TestU3_HTTPSCertUntrusted_NoFallback_TLSTrustFinding(t *testing.T) {
	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // never reached
	}))
	defer httpsSrv.Close()

	httpHits := 0
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHits++
	}))
	defer httpSrv.Close()

	// Bogus CA — cert chain will not verify.
	withTestBases(t, httpsSrv.URL, httpSrv.URL, writeBogusCA(t))

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 TLS-trust finding; got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("severity=%v want WARN", f.Severity)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("invariant_status=%v want INVARIANT_UNKNOWN", f.InvariantStatus)
	}
	wantPhrase := "TLS trust failure"
	if !strings.Contains(f.Summary, wantPhrase) {
		t.Errorf("summary must contain %q; got: %s", wantPhrase, f.Summary)
	}
	// Evidence must include the TLS error string.
	if len(f.Evidence) == 0 || f.Evidence[0].GetKeyValues()["tls_error"] == "" {
		t.Errorf("evidence must include tls_error metadata; got: %+v", f.Evidence)
	}
	if f.Evidence[0].GetKeyValues()["scheme"] != "https" {
		t.Errorf("evidence scheme must be 'https'; got: %q", f.Evidence[0].GetKeyValues()["scheme"])
	}
	if httpHits != 0 {
		t.Errorf("HTTP must NOT be probed when HTTPS cert fails; got %d HTTP hits", httpHits)
	}
}

// 4. HTTPS available + empty cluster list → ERROR finding, evidence
// scheme=https.
func TestU3_HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence(t *testing.T) {
	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/clusters" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{}) // empty
			return
		}
		http.NotFound(w, r)
	}))
	defer httpsSrv.Close()
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer httpSrv.Close()
	withTestBases(t, httpsSrv.URL, httpSrv.URL, writeCAPEM(t, httpsSrv))

	snap := mkSnap(mkScyllaInventory("active"))
	findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding; got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("severity=%v want ERROR", f.Severity)
	}
	if f.Evidence[0].GetKeyValues()["scheme"] != "https" {
		t.Errorf("evidence scheme must be 'https'; got: %q", f.Evidence[0].GetKeyValues()["scheme"])
	}
	if f.Evidence[0].GetKeyValues()["cluster_count"] != "0" {
		t.Errorf("evidence cluster_count must be '0'; got: %q",
			f.Evidence[0].GetKeyValues()["cluster_count"])
	}
}

// 5. HTTP-only legacy manager (no HTTPS listener at all) → falls back
// and reports HTTP outcome correctly. Tests both:
//   a. cluster exists → silent
//   b. cluster empty → ERROR with scheme=http and fallback_reason set
func TestU3_HTTPOnlyLegacy_SupportedDuringTransition(t *testing.T) {
	t.Run("cluster_exists_silent", func(t *testing.T) {
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/clusters" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": "legacy", "name": "globular-internal", "host": "10.0.0.99"},
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer httpSrv.Close()

		deadHTTPS := "https://" + allocateUnboundPort(t)
		withTestBases(t, deadHTTPS, httpSrv.URL, "/var/lib/globular/pki/ca.crt")

		snap := mkSnap(mkScyllaInventory("active"))
		findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
		if len(findings) != 0 {
			t.Errorf("HTTP-only legacy with cluster must be silent; got %d findings", len(findings))
		}
	})

	t.Run("cluster_empty_fires_with_http_evidence", func(t *testing.T) {
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{}) // empty
		}))
		defer httpSrv.Close()

		deadHTTPS := "https://" + allocateUnboundPort(t)
		withTestBases(t, deadHTTPS, httpSrv.URL, "/var/lib/globular/pki/ca.crt")

		snap := mkSnap(mkScyllaInventory("active"))
		findings := (scyllaManagerClusterRegistered{}).Evaluate(snap, testConfig())
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding from HTTP-only path; got %d", len(findings))
		}
		f := findings[0]
		if f.Evidence[0].GetKeyValues()["scheme"] != "http" {
			t.Errorf("evidence scheme must be 'http' on fallback; got %q",
				f.Evidence[0].GetKeyValues()["scheme"])
		}
		if reason := f.Evidence[0].GetKeyValues()["fallback_reason"]; reason == "" {
			t.Errorf("evidence must include fallback_reason for HTTP fallback; got: %+v",
				f.Evidence[0].GetKeyValues())
		}
	})
}

// Discovery: when the snapshot supplies a NodeRecord with AgentEndpoint,
// the rule must use that host instead of the hardcoded default. Otherwise
// the rule would be node-specific to globule-ryzen forever.
func TestU3_DiscoverHostFromSnapshot(t *testing.T) {
	host := discoverScyllaManagerHost(&collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-A", AgentEndpoint: "10.0.0.42:11000"},
			{NodeId: "node-B", AgentEndpoint: "10.0.0.43:11000"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-A": mkScyllaInventory("inactive"),
			"node-B": mkScyllaInventory("active"),
		},
	})
	if host != "10.0.0.43" {
		t.Errorf("discovery must pick node-B's host (10.0.0.43); got %q", host)
	}
}

// Discovery: when the snapshot has no Nodes or no matching active unit,
// discovery returns "" so the package-default bases stay in effect.
func TestU3_DiscoverHostFallback(t *testing.T) {
	cases := []*collector.Snapshot{
		nil,
		{},
		{Inventories: map[string]*node_agentpb.Inventory{"x": mkScyllaInventory("inactive")}},
		// Active but no NodeRecord → no match.
		{Inventories: map[string]*node_agentpb.Inventory{"x": mkScyllaInventory("active")}},
	}
	for i, snap := range cases {
		got := discoverScyllaManagerHost(snap)
		if got != "" {
			t.Errorf("case %d: expected empty discovery result; got %q", i, got)
		}
	}
}

// Compile-time guards keep the imports honest if the test file is later
// refactored away from these symbols.
var _ = x509.NewCertPool
