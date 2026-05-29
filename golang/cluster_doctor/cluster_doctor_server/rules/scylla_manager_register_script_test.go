package rules

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Project U.2 integration tests for /usr/lib/globular/bin/scylla-manager-register-cluster.
//
// These tests exercise the installed package script with mocked HTTPS / HTTP
// endpoints. They skip when the script is not installed (typical of a
// non-cluster-node CI environment); on the deployed cluster they provide
// regression coverage for the HTTPS-first / HTTP-fallback / fail-closed
// behavior the spec calls out.
//
// The script honors these env vars:
//   SCYLLA_MANAGER_CLUSTER_NAME   target cluster name (default: globular-internal)
//   SCYLLA_MANAGER_HOST           host placeholder used in the default base URLs
//   SCYLLA_MANAGER_HTTPS_BASE     full override of the HTTPS read URL base
//   SCYLLA_MANAGER_HTTP_BASE      full override of the HTTP read URL base
//   SCYLLA_AGENT_CFG              path to the agent yaml (for the WRITE phase)
//   SCYLLA_CFG                    path to scylla.yaml (for host detection)
//   GLOBULAR_CA                   CA file used by the HTTPS probe
//
// Tests set these to point at httptest servers + tempfiles so the script
// runs in full hermetic mode.

const registerScriptPath = "/usr/lib/globular/bin/scylla-manager-register-cluster"

func skipIfScriptNotInstalled(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(registerScriptPath); err != nil {
		t.Skipf("script not installed at %s — Project U.2 integration tests skipped", registerScriptPath)
	}
}

// writeCABundle writes the test TLS server's cert to a temp file
// formatted as a PEM bundle suitable for curl --cacert.
func writeCABundle(t *testing.T, srv *httptest.Server) string {
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

// 1. HTTPS available and trusted → script chooses HTTPS for read-side probes.
func TestRegisterScript_HTTPSReachable_PrefersHTTPS(t *testing.T) {
	skipIfScriptNotInstalled(t)

	clusters := []map[string]any{
		{"id": "test-cluster-1", "name": "globular-internal", "host": "10.0.0.99"},
	}
	httpsHit := 0
	httpHit := 0
	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpsHit++
		switch r.URL.Path {
		case "/api/v1/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.10.1"})
		case "/api/v1/clusters":
			_ = json.NewEncoder(w).Encode(clusters)
		default:
			http.NotFound(w, r)
		}
	}))
	defer httpsSrv.Close()
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHit++
		// If reached, the script wrongly fell back. Return data anyway so
		// the test fails on the counter check rather than a script crash.
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer httpSrv.Close()

	caPath := writeCABundle(t, httpsSrv)
	output := runRegisterScript(t, map[string]string{
		"SCYLLA_MANAGER_HTTPS_BASE": httpsSrv.URL + "/api/v1",
		"SCYLLA_MANAGER_HTTP_BASE":  httpSrv.URL + "/api/v1",
		"GLOBULAR_CA":               caPath,
	})

	if !strings.Contains(output, "HTTPS reachable with valid cert") {
		t.Errorf("script should log HTTPS preference; got:\n%s", output)
	}
	if !strings.Contains(output, "already registered — no-op") {
		t.Errorf("script should detect existing cluster idempotently; got:\n%s", output)
	}
	if httpsHit == 0 {
		t.Error("HTTPS endpoint was never hit — script did not prefer HTTPS")
	}
	if httpHit > 0 {
		t.Errorf("HTTP endpoint should not be hit when HTTPS is reachable; was hit %d times", httpHit)
	}
}

// 2. HTTPS connection refused → script falls back to HTTP.
func TestRegisterScript_HTTPSConnectionRefused_FallsBackToHTTP(t *testing.T) {
	skipIfScriptNotInstalled(t)

	httpHit := 0
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHit++
		switch r.URL.Path {
		case "/api/v1/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.10.1"})
		case "/api/v1/clusters":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "test-cluster-2", "name": "globular-internal", "host": "10.0.0.99"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer httpSrv.Close()

	// Allocate but never bind an HTTPS port → curl exit 7 (connection
	// refused). Use a fresh listener+close to claim a free port, then
	// release it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadHTTPS := "https://" + l.Addr().String() + "/api/v1"
	_ = l.Close()

	output := runRegisterScript(t, map[string]string{
		"SCYLLA_MANAGER_HTTPS_BASE": deadHTTPS,
		"SCYLLA_MANAGER_HTTP_BASE":  httpSrv.URL + "/api/v1",
		"GLOBULAR_CA":               "/var/lib/globular/pki/ca.crt", // any path; curl never reaches it
	})

	if !strings.Contains(output, "HTTPS not enabled") {
		t.Errorf("script should log HTTPS connection-refused fallback; got:\n%s", output)
	}
	if !strings.Contains(output, "falling back to "+httpSrv.URL) {
		t.Errorf("script should log HTTP fallback target; got:\n%s", output)
	}
	if httpHit == 0 {
		t.Error("HTTP endpoint was never hit — script did not fall back")
	}
}

// 3. HTTPS reachable but cert validation fails → script fails CLOSED.
// Critical for the security contract: we do not silently downgrade.
func TestRegisterScript_HTTPSCertInvalid_FailsClosed(t *testing.T) {
	skipIfScriptNotInstalled(t)

	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.10.1"})
	}))
	defer httpsSrv.Close()

	// Provide a CA file that does NOT contain the test server's cert.
	// The cert is real but signed by a CA the script does not trust.
	pool := x509.NewCertPool()
	pool.AddCert(&x509.Certificate{}) // bogus empty cert
	dir := t.TempDir()
	caPath := filepath.Join(dir, "wrong-ca.crt")
	if err := os.WriteFile(caPath, []byte("-----BEGIN CERTIFICATE-----\nINVALID\n-----END CERTIFICATE-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	httpHit := 0
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHit++
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer httpSrv.Close()

	out, exitCode := runRegisterScriptCheckExit(t, map[string]string{
		"SCYLLA_MANAGER_HTTPS_BASE": httpsSrv.URL + "/api/v1",
		"SCYLLA_MANAGER_HTTP_BASE":  httpSrv.URL + "/api/v1",
		"GLOBULAR_CA":               caPath,
	})

	if exitCode == 0 {
		t.Errorf("script must exit non-zero on cert validation failure; got 0\noutput:\n%s", out)
	}
	if !strings.Contains(out, "cert validation failed") && !strings.Contains(out, "refusing to fall back") {
		t.Errorf("script must log clear cert-validation-failure reason; got:\n%s", out)
	}
	if httpHit > 0 {
		t.Errorf("script must NOT fall back to HTTP on cert validation failure; HTTP hit %d times", httpHit)
	}
}

// 4. Idempotency over HTTPS — existing cluster by name → no-op.
// Same as test 1 but explicit; documents the contract from spec item 4.
func TestRegisterScript_ExistingClusterByName_NoOp(t *testing.T) {
	skipIfScriptNotInstalled(t)

	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.10.1"})
		case "/api/v1/clusters":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "x", "name": "globular-internal", "host": "10.0.0.99"},
			})
		}
	}))
	defer httpsSrv.Close()

	caPath := writeCABundle(t, httpsSrv)
	out, exitCode := runRegisterScriptCheckExit(t, map[string]string{
		"SCYLLA_MANAGER_HTTPS_BASE": httpsSrv.URL + "/api/v1",
		"SCYLLA_MANAGER_HTTP_BASE":  "http://127.0.0.1:0/api/v1",
		"GLOBULAR_CA":               caPath,
	})
	if exitCode != 0 {
		t.Errorf("existing-cluster path must exit 0; got %d\noutput:\n%s", exitCode, out)
	}
	if !strings.Contains(out, "already registered — no-op") {
		t.Errorf("script must log idempotent skip; got:\n%s", out)
	}
}

// 5. Missing cluster — script proceeds to read agent token and call sctool.
// We don't have sctool in test PATH, so we substitute one via a custom
// PATH that has a stub. The stub captures its argv; the test asserts the
// stub was invoked with --api-url pointing at the HTTP base (write path
// stays on HTTP per the spec).
func TestRegisterScript_MissingCluster_UsesHTTPForWritePath(t *testing.T) {
	skipIfScriptNotInstalled(t)

	httpsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.10.1"})
		case "/api/v1/clusters":
			// Empty cluster list → script proceeds to register.
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer httpsSrv.Close()

	caPath := writeCABundle(t, httpsSrv)

	// Stub agent yaml: write a token + an HTTPS port for the agent.
	dir := t.TempDir()
	agentCfg := filepath.Join(dir, "scylla-manager-agent.yaml")
	if err := os.WriteFile(agentCfg, []byte("auth_token: test-token-xyz\nhttps: 10.0.0.99:5612\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scyllaCfg := filepath.Join(dir, "scylla.yaml")
	if err := os.WriteFile(scyllaCfg, []byte("rpc_address: 10.0.0.99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stub sctool: a small shell script that captures its argv.
	stubLog := filepath.Join(dir, "sctool.log")
	stub := filepath.Join(dir, "sctool")
	stubBody := "#!/bin/sh\necho \"$@\" > " + stubLog + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(stubBody), 0o755); err != nil {
		t.Fatal(err)
	}

	out, exitCode := runRegisterScriptCheckExit(t, map[string]string{
		"SCYLLA_MANAGER_HTTPS_BASE": httpsSrv.URL + "/api/v1",
		"SCYLLA_MANAGER_HTTP_BASE":  "http://10.0.0.99:5080/api/v1",
		"SCYLLA_AGENT_CFG":          agentCfg,
		"SCYLLA_CFG":                scyllaCfg,
		"GLOBULAR_CA":               caPath,
		"PATH":                      dir + ":" + os.Getenv("PATH"),
	})
	if exitCode != 0 {
		t.Errorf("missing-cluster path must exit 0 on success; got %d\noutput:\n%s", exitCode, out)
	}
	argv, _ := os.ReadFile(stubLog)
	if !strings.Contains(string(argv), "--api-url http://10.0.0.99:5080/api/v1") {
		t.Errorf("sctool must be invoked with HTTP API URL (write path stays HTTP); got argv: %s", argv)
	}
	if !strings.Contains(string(argv), "cluster add") {
		t.Errorf("sctool must be invoked with 'cluster add'; got: %s", argv)
	}
	// Read path must still have been HTTPS — verified by the cert-required URL.
	if strings.Contains(out, "falling back") {
		t.Errorf("script should NOT have fallen back when HTTPS works; got:\n%s", out)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func runRegisterScript(t *testing.T, env map[string]string) string {
	t.Helper()
	out, _ := runRegisterScriptCheckExit(t, env)
	return out
}

func runRegisterScriptCheckExit(t *testing.T, env map[string]string) (string, int) {
	t.Helper()
	cmd := exec.Command(registerScriptPath)
	envSlice := os.Environ()
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}
	cmd.Env = envSlice
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return string(out), exitCode
}

// Ensure the test package compiles with crypto/tls referenced (used
// indirectly via httptest.NewTLSServer's auto-cert generation).
var _ = tls.VersionTLS12
