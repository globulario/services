// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=scylladb_manager_cluster_registration_rule
// @awareness risk=medium
package rules

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Project U.3 — HTTPS-first probe for scylla-manager's read API.
//
// The doctor used to hit only `http://10.0.0.63:5080`. With Project U.1 the
// manager now also serves HTTPS on :5443 with the Globular service cert.
// This file now:
//
//   1. Discovers the scylla-manager host from the snapshot (the node where
//      globular-scylla-manager.service is active), with a fallback to the
//      legacy hardcoded constant so the rule continues to function during
//      transition / on snapshots that omit NodeRecord agent endpoints.
//   2. Probes HTTPS first using a TLS config that trusts ONLY the Globular
//      CA — the system trust store is intentionally not loaded (parallel to
//      the registration script's `--capath /dev/null --cacert` requirement
//      shipped in scylla-manager 1.2.75).
//   3. Falls back to HTTP only when HTTPS is connection-refused (listener
//      absent). Any TLS / cert validation failure is reported as an
//      inconclusive WARN finding — never silently downgraded — so the
//      misconfiguration is visible.
//
// HTTP-only legacy managers (no :5443 listener) are still supported. The
// cluster-registered ERROR finding fires when HTTPS confirms an empty
// cluster list OR when HTTPS is unavailable and HTTP confirms an empty
// list. Either way, the evidence records which scheme was used and (if
// HTTP) why.

// Defaults — overridable from tests. The legacy hardcoded IP remains the
// fallback when host discovery returns empty.
var (
	scyllaManagerHTTPSBase = "https://10.0.0.63:5443"
	scyllaManagerHTTPBase  = "http://10.0.0.63:5080"
	scyllaManagerCAPath    = "/var/lib/globular/pki/ca.crt"
)

// scyllaManagerHTTPClient is reused for HTTP (no TLS).
var scyllaManagerHTTPClient = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
	},
}

// newScyllaManagerHTTPSClient builds an https.Client whose ONLY trust
// anchor is the Globular CA at scyllaManagerCAPath. The system bundle is
// intentionally not loaded — without this, an OS-trusted CA could
// accidentally validate a wrong-cert scylla-manager listener and defeat
// the strict-verification guarantee. Returns nil + error when the CA file
// cannot be read; caller should treat that as inconclusive (don't probe).
func newScyllaManagerHTTPSClient() (*http.Client, error) {
	caBytes, err := os.ReadFile(scyllaManagerCAPath)
	if err != nil {
		return nil, fmt.Errorf("read CA at %s: %w", scyllaManagerCAPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("CA file %s contained no usable PEM blocks", scyllaManagerCAPath)
	}
	return &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				MinVersion: tls.VersionTLS12,
			},
		},
	}, nil
}

// probeOutcome captures the structured result of a probe so the rule can
// distinguish HTTPS / HTTP / TLS-failure paths without re-parsing errors.
type probeOutcome struct {
	scheme         string // "https" | "http" | ""
	clusters       []map[string]any
	httpsTLSErr    error  // non-nil iff HTTPS TLS verification failed
	fallbackReason string // populated when scheme==http and HTTPS was attempted
	transportErr   error  // any other transport error (treated as inconclusive)
}

// probeScyllaManager runs the HTTPS-first probe. Tests can override the
// two URL bases via the package-level vars.
func probeScyllaManager(ctx context.Context) probeOutcome {
	out := probeOutcome{}

	httpsClient, caErr := newScyllaManagerHTTPSClient()
	if caErr == nil {
		clusters, err := fetchScyllaManagerClustersWith(ctx, httpsClient, scyllaManagerHTTPSBase)
		switch {
		case err == nil:
			out.scheme = "https"
			out.clusters = clusters
			return out
		case isTLSVerificationError(err):
			// Strict-verification failure — do NOT fall back. Surface as
			// inconclusive WARN finding upstream.
			out.httpsTLSErr = err
			return out
		case isHTTPSUnavailableError(err):
			// HTTPS listener absent (conn refused / dial timeout / no route).
			// Safe fallback to HTTP.
			out.fallbackReason = fmt.Sprintf("https_unavailable: %v", err)
		default:
			// Other transport-level error against HTTPS — also fall back to
			// HTTP, but record the reason so the evidence is honest.
			out.fallbackReason = fmt.Sprintf("https_error: %v", err)
		}
	} else {
		// Can't build the HTTPS client (CA missing/unreadable) — try HTTP.
		out.fallbackReason = fmt.Sprintf("ca_unavailable: %v", caErr)
	}

	clusters, err := fetchScyllaManagerClustersWith(ctx, scyllaManagerHTTPClient, scyllaManagerHTTPBase)
	if err != nil {
		out.transportErr = err
		return out
	}
	out.scheme = "http"
	out.clusters = clusters
	return out
}

// isTLSVerificationError returns true when the error is a strict-CA
// verification failure (cert chain not trusted, hostname mismatch, expired,
// etc.) — the cases where we MUST NOT silently downgrade.
func isTLSVerificationError(err error) bool {
	if err == nil {
		return false
	}
	var tlsCertErr *tls.CertificateVerificationError
	var unkAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalidErr x509.CertificateInvalidError
	if errors.As(err, &tlsCertErr) ||
		errors.As(err, &unkAuth) ||
		errors.As(err, &hostErr) ||
		errors.As(err, &invalidErr) {
		return true
	}
	// Defensive string match for older error shapes / wrappers that don't
	// expose the typed cert error through errors.As.
	es := err.Error()
	return strings.Contains(es, "x509:") || strings.Contains(es, "tls: failed to verify")
}

// isHTTPSUnavailableError returns true when the error indicates the HTTPS
// listener is simply absent — the case where falling back to HTTP is safe.
func isHTTPSUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// Some platforms wrap conn-refused in net.OpError without exposing
	// ECONNREFUSED via errors.Is — string match as a safety net.
	es := err.Error()
	return strings.Contains(es, "connection refused") ||
		strings.Contains(es, "no route to host") ||
		strings.Contains(es, "network is unreachable")
}

// scyllaManagerClusterRegistered fires on the two failure modes the
// invariant guards against:
//
//   - clusters list is empty (no cluster registered with scylla-manager)
//     — emits an ERROR finding identifying the backup-readiness gap.
//   - HTTPS probe succeeded the TCP/HTTP layer but failed TLS verification
//     — emits a WARN finding so the operator notices a trust-chain bug
//     rather than the doctor silently downgrading to HTTP.
//
// Pure transport errors (conn refused on both schemes, timeouts, etc.)
// remain silent: another rule covers the unit-down case and double-
// reporting transient network blips during a snapshot would add noise.
type scyllaManagerClusterRegistered struct{}

func (scyllaManagerClusterRegistered) ID() string       { return "scylla_manager.cluster_registered" }
func (scyllaManagerClusterRegistered) Category() string { return "infrastructure" }
func (scyllaManagerClusterRegistered) Scope() string    { return "cluster" }

func (r scyllaManagerClusterRegistered) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if !anyNodeRunsScyllaManager(snap) {
		return nil
	}

	// Refresh the URL bases from snapshot-derived host where possible.
	// On miss, the package-level defaults remain in effect.
	applyDiscoveredHost(snap)

	outcome := probeScyllaManager(context.Background())

	if outcome.httpsTLSErr != nil {
		return []Finding{newScyllaManagerTLSTrustFinding(outcome.httpsTLSErr)}
	}
	if outcome.scheme == "" {
		// Both probes failed for non-TLS reasons. Inconclusive.
		return nil
	}
	if len(outcome.clusters) == 0 {
		return []Finding{newScyllaManagerUnregisteredFinding(outcome.scheme, outcome.fallbackReason)}
	}
	return nil
}

// applyDiscoveredHost rewrites the package-level URL bases to point at
// the node currently running scylla-manager, when the snapshot identifies
// it. Idempotent and silent on miss. Tests that override the bases
// directly should not pass a populated snapshot OR should re-override
// after Evaluate runs (typical pattern: call withTestEndpoint after the
// test snapshot is built).
func applyDiscoveredHost(snap *collector.Snapshot) {
	host := discoverScyllaManagerHost(snap)
	if host == "" {
		return
	}
	scyllaManagerHTTPSBase = "https://" + host + ":5443"
	scyllaManagerHTTPBase = "http://" + host + ":5080"
}

// discoverScyllaManagerHost walks the snapshot and returns the host
// portion of NodeRecord.AgentEndpoint for the node whose inventory
// reports globular-scylla-manager.service active. Returns "" when no
// match is found (snapshot lacks Nodes, lacks Inventories, or no node
// is running it). Empty result lets the caller fall back to the package
// default (Project U.3 transition behavior).
func discoverScyllaManagerHost(snap *collector.Snapshot) string {
	if snap == nil {
		return ""
	}
	// Build nodeId → host map from snapshot.Nodes
	nodeHost := map[string]string{}
	for _, n := range snap.Nodes {
		if n == nil {
			continue
		}
		endpoint := n.GetAgentEndpoint()
		if endpoint == "" {
			continue
		}
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil || host == "" {
			continue
		}
		nodeHost[n.GetNodeId()] = host
	}
	for nodeID, inv := range snap.Inventories {
		if inv == nil {
			continue
		}
		for _, u := range inv.GetUnits() {
			if !strings.EqualFold(u.GetName(), "globular-scylla-manager.service") {
				continue
			}
			if !isActiveUnitState(u) {
				continue
			}
			if host, ok := nodeHost[nodeID]; ok {
				return host
			}
		}
	}
	return ""
}

// anyNodeRunsScyllaManager reports whether at least one node's inventory
// shows globular-scylla-manager.service in an active state.
func anyNodeRunsScyllaManager(snap *collector.Snapshot) bool {
	if snap == nil {
		return false
	}
	for _, inv := range snap.Inventories {
		if inv == nil {
			continue
		}
		for _, u := range inv.GetUnits() {
			if !strings.EqualFold(u.GetName(), "globular-scylla-manager.service") {
				continue
			}
			if isActiveUnitState(u) {
				return true
			}
		}
	}
	return false
}

func isActiveUnitState(u *node_agentpb.UnitStatus) bool {
	if u == nil {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(u.GetState()))
	return state == "active" || strings.HasPrefix(state, "active")
}

// fetchScyllaManagerClustersWith probes /api/v1/clusters via the supplied
// client and base URL.
func fetchScyllaManagerClustersWith(ctx context.Context, client *http.Client, base string) ([]map[string]any, error) {
	url := strings.TrimRight(base, "/") + "/api/v1/clusters"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scylla-manager /api/v1/clusters returned status %d", resp.StatusCode)
	}
	var clusters []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&clusters); err != nil {
		return nil, fmt.Errorf("decode /api/v1/clusters: %w", err)
	}
	return clusters, nil
}

func newScyllaManagerUnregisteredFinding(scheme, fallbackReason string) Finding {
	const id = "scylla_manager.cluster_registered"
	summary := "scylla-manager is running but no Scylla cluster is registered " +
		"(backup, repair, and restore are unavailable until `sctool cluster add` runs)"
	endpoint := scyllaManagerHTTPSBase
	if scheme == "http" {
		endpoint = scyllaManagerHTTPBase
	}
	kv := map[string]string{
		"endpoint":      endpoint,
		"scheme":        scheme,
		"cluster_count": "0",
	}
	if fallbackReason != "" {
		kv["fallback_reason"] = fallbackReason
	}
	return Finding{
		FindingID:       FindingID(id, "globular-scylla-manager", "no_cluster_registered"),
		InvariantID:     id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:        "infrastructure",
		EntityRef:       "globular-scylla-manager.service",
		Summary:         summary,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("scylla_manager.http", "GET /api/v1/clusters", kv),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Run the package-shipped registration script (Project S): "+
					"/usr/lib/globular/bin/scylla-manager-register-cluster",
				""),
			step(2,
				"Or register manually: `sctool cluster add --host <scylla-ip> "+
					"--port <agent-https-port> --name globular-internal "+
					"--auth-token <from-scylla-manager-agent.yaml>`",
				""),
		},
	}
}

func newScyllaManagerTLSTrustFinding(tlsErr error) Finding {
	const id = "scylla_manager.cluster_registered"
	summary := "scylla-manager HTTPS endpoint reachable but TLS trust failure blocks safe verification " +
		"(refusing to fall back to HTTP; cluster registration state cannot be confirmed)"
	return Finding{
		FindingID:       FindingID(id, "globular-scylla-manager", "tls_trust_failure"),
		InvariantID:     id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:        "infrastructure",
		EntityRef:       "globular-scylla-manager.service",
		Summary:         summary,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("scylla_manager.https", "GET /api/v1/clusters", map[string]string{
				"endpoint":  scyllaManagerHTTPSBase,
				"scheme":    "https",
				"tls_error": tlsErr.Error(),
				"ca_path":   scyllaManagerCAPath,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Verify the scylla-manager process is using the Globular service cert "+
					"(tls_cert_file in /var/lib/globular/scylla-manager/scylla-manager.yaml)",
				""),
			step(2,
				"Verify the Globular CA at "+scyllaManagerCAPath+
					" trusts the scylla-manager service cert (openssl verify -CAfile)",
				""),
			step(3,
				"If the certificate is correct, restart globular-scylla-manager.service "+
					"to reload it",
				"systemctl restart globular-scylla-manager.service"),
		},
	}
}

// Compile-time guard: NodeRecord must expose AgentEndpoint. If the proto
// changes we want to know at build time, not at runtime.
var _ = (*cluster_controllerpb.NodeRecord)(nil).GetAgentEndpoint
