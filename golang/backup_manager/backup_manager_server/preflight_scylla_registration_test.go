package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

func TestReadScyllaManagerEndpointPrefersHTTPS(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "scylla-manager.yaml")
	writeTestFile(t, cfg, "http: 10.0.0.63:5080\nhttps: 10.0.0.63:5443\n")

	withScyllaConfigPaths(t, []string{cfg}, nil)

	endpoint := readScyllaManagerEndpoint()
	if endpoint.URL != "https://10.0.0.63:5443" {
		t.Fatalf("endpoint URL = %q, want https://10.0.0.63:5443", endpoint.URL)
	}
	if endpoint.Scheme != "https" {
		t.Fatalf("endpoint scheme = %q, want https", endpoint.Scheme)
	}
}

func TestReadScyllaManagerEndpointFallsBackToHTTP(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "scylla-manager.yaml")
	writeTestFile(t, cfg, "http: 10.0.0.63:5080\n")

	withScyllaConfigPaths(t, []string{cfg}, nil)

	endpoint := readScyllaManagerEndpoint()
	if endpoint.URL != "http://10.0.0.63:5080" {
		t.Fatalf("endpoint URL = %q, want http://10.0.0.63:5080", endpoint.URL)
	}
	if endpoint.Scheme != "http" {
		t.Fatalf("endpoint scheme = %q, want http", endpoint.Scheme)
	}
}

func TestTryRegisterScyllaRefusesWithoutManagerEndpoint(t *testing.T) {
	withScyllaConfigPaths(t, []string{filepath.Join(t.TempDir(), "missing.yaml")}, nil)
	restoreLookPath := stubExecLookPath(t, "/usr/bin/sctool", nil)
	defer restoreLookPath()

	var dials []string
	restoreDial := stubDialTimeout(t, func(network, address string, timeout time.Duration) (net.Conn, error) {
		dials = append(dials, address)
		return nil, errors.New("unexpected dial")
	})
	defer restoreDial()

	logs, restoreLog := captureSlog(t)
	defer restoreLog()

	srv := &server{}
	if ok := srv.tryRegisterScylla(); ok {
		t.Fatalf("tryRegisterScylla returned true, want false")
	}
	if len(dials) != 0 {
		t.Fatalf("unexpected dial attempts: %v", dials)
	}
	got := logs.String()
	if strings.Contains(got, "127.0.0.1:5080") {
		t.Fatalf("logs must not mention stale loopback endpoint: %s", got)
	}
	if !strings.Contains(got, "scylla_manager.registration.skipped.manager_unreachable") {
		t.Fatalf("expected manager_unreachable log, got: %s", got)
	}
}

func TestTryRegisterScyllaUnreadableAgentConfigLogsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	managerCfg := filepath.Join(dir, "scylla-manager.yaml")
	agentPath := filepath.Join(dir, "scylla-manager-agent.yaml")
	logPath := filepath.Join(dir, "sctool.log")
	writeTestFile(t, managerCfg, "https: 10.0.0.63:5443\n")
	writeExecutable(t, filepath.Join(dir, "sctool"), preflightSctoolScript(logPath))

	withScyllaConfigPaths(t, []string{managerCfg}, []string{agentPath})
	restorePath := prependPath(t, dir)
	defer restorePath()
	restoreLookPath := stubExecLookPath(t, "", nil)
	defer restoreLookPath()
	restoreNative := stubNativeScyllaDetector(t, "10.0.0.63", "globular")
	defer restoreNative()
	restoreDial := stubDialTimeout(t, func(network, address string, timeout time.Duration) (net.Conn, error) {
		if address == "10.0.0.63:5443" {
			return &fakeConn{}, nil
		}
		return nil, errors.New("refused")
	})
	defer restoreDial()

	logs, restoreLog := captureSlog(t)
	defer restoreLog()

	srv := &server{}
	if ok := srv.tryRegisterScylla(); ok {
		t.Fatalf("tryRegisterScylla returned true, want false")
	}
	got := logs.String()
	if !strings.Contains(got, "scylla_manager.registration.skipped.agent_config_unreadable") {
		t.Fatalf("expected agent_config_unreadable log, got: %s", got)
	}
	for _, want := range []string{agentPath, "owner=", "group=", "mode="} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in log output: %s", want, got)
		}
	}
}

func TestTryRegisterScyllaSkipsWhenAuthTokenMissing(t *testing.T) {
	dir := t.TempDir()
	managerCfg := filepath.Join(dir, "scylla-manager.yaml")
	agentCfg := filepath.Join(dir, "scylla-manager-agent.yaml")
	logPath := filepath.Join(dir, "sctool.log")
	writeTestFile(t, managerCfg, "https: 10.0.0.63:5443\n")
	writeTestFile(t, agentCfg, "https: 10.0.0.63:5612\n")
	writeExecutable(t, filepath.Join(dir, "sctool"), preflightSctoolScript(logPath))

	withScyllaConfigPaths(t, []string{managerCfg}, []string{agentCfg})
	restorePath := prependPath(t, dir)
	defer restorePath()
	restoreLookPath := stubExecLookPath(t, "", nil)
	defer restoreLookPath()
	restoreNative := stubNativeScyllaDetector(t, "10.0.0.63", "globular")
	defer restoreNative()
	restoreDial := stubDialTimeout(t, func(network, address string, timeout time.Duration) (net.Conn, error) {
		if address == "10.0.0.63:5443" {
			return &fakeConn{}, nil
		}
		return nil, errors.New("refused")
	})
	defer restoreDial()

	logs, restoreLog := captureSlog(t)
	defer restoreLog()

	srv := &server{}
	if ok := srv.tryRegisterScylla(); ok {
		t.Fatalf("tryRegisterScylla returned true, want false")
	}
	got := logs.String()
	if !strings.Contains(got, "scylla_manager.registration.skipped.agent_token_missing") {
		t.Fatalf("expected agent_token_missing log, got: %s", got)
	}
	if strings.Contains(got, "cluster add") {
		t.Fatalf("cluster add must not run when token is missing: %s", got)
	}
}

func TestRealRegisteredClustersIgnoresHints(t *testing.T) {
	clusters := realRegisteredClusters([]string{"native:globular", "scylla_host:10.0.0.63"})
	if len(clusters) != 0 {
		t.Fatalf("realRegisteredClusters = %v, want none", clusters)
	}
}

func TestTryRegisterScyllaRegistersZeroRealClusters(t *testing.T) {
	dir := t.TempDir()
	managerCfg := filepath.Join(dir, "scylla-manager.yaml")
	agentCfg := filepath.Join(dir, "scylla-manager-agent.yaml")
	logPath := filepath.Join(dir, "sctool.log")
	writeTestFile(t, managerCfg, "https: 10.0.0.63:5443\n")
	writeTestFile(t, agentCfg, "auth_token: test-token\nhttps: 10.0.0.63:5612\n")
	writeExecutable(t, filepath.Join(dir, "sctool"), registerSctoolScript(logPath))

	withScyllaConfigPaths(t, []string{managerCfg}, []string{agentCfg})
	restorePath := prependPath(t, dir)
	defer restorePath()
	restoreLookPath := stubExecLookPath(t, "", nil)
	defer restoreLookPath()
	restoreNative := stubNativeScyllaDetector(t, "10.0.0.63", "globular-native")
	defer restoreNative()
	restoreDial := stubDialTimeout(t, func(network, address string, timeout time.Duration) (net.Conn, error) {
		switch address {
		case "10.0.0.63:5443", "10.0.0.63:5612":
			return &fakeConn{}, nil
		default:
			return nil, errors.New("refused")
		}
	})
	defer restoreDial()

	logs, restoreLog := captureSlog(t)
	defer restoreLog()

	srv := &server{
		Domain:   "globular.example",
		CertFile: "/tmp/service.crt",
		KeyFile:  "/tmp/service.key",
	}
	if ok := srv.tryRegisterScylla(); !ok {
		t.Fatalf("tryRegisterScylla returned false, want true")
	}
	callLog := readFile(t, logPath)
	if !strings.Contains(callLog, "cluster add") {
		t.Fatalf("expected cluster add invocation, got: %s", callLog)
	}
	for _, want := range []string{
		"--api-url https://10.0.0.63:5443/api/v1",
		"--host 10.0.0.63",
		"--name globular.example",
		"--auth-token test-token",
		"--port 5612",
	} {
		if !strings.Contains(callLog, want) {
			t.Fatalf("expected %q in sctool log: %s", want, callLog)
		}
	}
	if !strings.Contains(logs.String(), "scylla_manager.registration.succeeded") {
		t.Fatalf("expected success log, got: %s", logs.String())
	}
}

func TestTryRegisterScyllaDoesNotAddWhenClusterAlreadyRegistered(t *testing.T) {
	dir := t.TempDir()
	managerCfg := filepath.Join(dir, "scylla-manager.yaml")
	logPath := filepath.Join(dir, "sctool.log")
	writeTestFile(t, managerCfg, "https: 10.0.0.63:5443\n")
	writeExecutable(t, filepath.Join(dir, "sctool"), registeredSctoolScript(logPath))

	withScyllaConfigPaths(t, []string{managerCfg}, nil)
	restorePath := prependPath(t, dir)
	defer restorePath()
	restoreLookPath := stubExecLookPath(t, "", nil)
	defer restoreLookPath()
	restoreDial := stubDialTimeout(t, func(network, address string, timeout time.Duration) (net.Conn, error) {
		if address == "10.0.0.63:5443" {
			return &fakeConn{}, nil
		}
		return nil, errors.New("refused")
	})
	defer restoreDial()

	logs, restoreLog := captureSlog(t)
	defer restoreLog()

	srv := &server{}
	if ok := srv.tryRegisterScylla(); !ok {
		t.Fatalf("tryRegisterScylla returned false, want true")
	}
	callLog := readFile(t, logPath)
	if strings.Contains(callLog, "cluster add") {
		t.Fatalf("cluster add must not run for existing cluster: %s", callLog)
	}
	if !strings.Contains(logs.String(), "scylla_manager.registration.already_registered") {
		t.Fatalf("expected already_registered log, got: %s", logs.String())
	}
}

func TestPreflightCheckDoesNotAutoRegisterScylla(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "tool.log")
	writeExecutable(t, filepath.Join(dir, "sctool"), preflightSctoolScript(logPath))
	for _, tool := range []string{"etcdctl", "restic", "rclone", "sha256sum"} {
		writeExecutable(t, filepath.Join(dir, tool), "#!/bin/sh\nexit 0\n")
	}

	restorePath := prependPath(t, dir)
	defer restorePath()
	restoreLookPath := stubExecLookPath(t, "", nil)
	defer restoreLookPath()
	restoreNative := stubNativeScyllaDetector(t, "10.0.0.63", "globular-native")
	defer restoreNative()

	srv := &server{}
	resp, err := srv.PreflightCheck(context.Background(), &backup_managerpb.PreflightCheckRequest{})
	if err != nil {
		t.Fatalf("PreflightCheck error: %v", err)
	}
	if resp == nil {
		t.Fatal("PreflightCheck returned nil response")
	}
	callLog := readFile(t, logPath)
	if strings.Contains(callLog, "cluster add") {
		t.Fatalf("PreflightCheck must not auto-register Scylla: %s", callLog)
	}
}

func TestInstallDay0StillDefersRegistration(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "release", "install-day0.sh"))
	if err != nil {
		t.Fatalf("read install-day0.sh: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Scylla cluster registration deferred to backup_manager") {
		t.Fatalf("expected deferral log line in install-day0.sh")
	}
	if !strings.Contains(text, "Do NOT run `sctool cluster add` here.") {
		t.Fatalf("expected explicit no-cluster-add contract in install-day0.sh")
	}
}

type fakeConn struct{}

func (fakeConn) Read([]byte) (int, error)         { return 0, nil }
func (fakeConn) Write([]byte) (int, error)        { return 0, nil }
func (fakeConn) Close() error                     { return nil }
func (fakeConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (fakeConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (fakeConn) SetDeadline(time.Time) error      { return nil }
func (fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (fakeConn) SetWriteDeadline(time.Time) error { return nil }

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func withScyllaConfigPaths(t *testing.T, managerPaths, agentPaths []string) {
	t.Helper()
	prevManager := append([]string(nil), scyllaManagerConfigPaths...)
	prevAgent := append([]string(nil), scyllaAgentConfigPaths...)
	if managerPaths != nil {
		scyllaManagerConfigPaths = append([]string(nil), managerPaths...)
	}
	if agentPaths != nil {
		scyllaAgentConfigPaths = append([]string(nil), agentPaths...)
	}
	t.Cleanup(func() {
		scyllaManagerConfigPaths = prevManager
		scyllaAgentConfigPaths = prevAgent
	})
}

func captureSlog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return buf, func() { slog.SetDefault(prev) }
}

func stubExecLookPath(t *testing.T, defaultPath string, paths map[string]string) func() {
	t.Helper()
	prev := execLookPath
	execLookPath = func(file string) (string, error) {
		if paths != nil {
			if path, ok := paths[file]; ok {
				return path, nil
			}
		}
		if defaultPath != "" {
			return defaultPath, nil
		}
		return exec.LookPath(file)
	}
	return func() { execLookPath = prev }
}

func stubDialTimeout(t *testing.T, fn func(network, address string, timeout time.Duration) (net.Conn, error)) func() {
	t.Helper()
	prev := dialTimeout
	dialTimeout = fn
	return func() { dialTimeout = prev }
}

func stubNativeScyllaDetector(t *testing.T, host, cluster string) func() {
	t.Helper()
	prev := nativeScyllaDBDetector
	nativeScyllaDBDetector = func() (string, string) { return host, cluster }
	return func() { nativeScyllaDBDetector = prev }
}

func prependPath(t *testing.T, dir string) func() {
	t.Helper()
	prev := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+prev); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	return func() {
		_ = os.Setenv("PATH", prev)
	}
}

func registerSctoolScript(logPath string) string {
	return "#!/bin/sh\n" +
		"echo \"$@\" >> " + logPath + "\n" +
		"if [ \"$1\" = \"cluster\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  COUNT_FILE=" + logPath + ".count\n" +
		"  COUNT=0\n" +
		"  if [ -f \"$COUNT_FILE\" ]; then\n" +
		"    COUNT=$(cat \"$COUNT_FILE\")\n" +
		"  fi\n" +
		"  COUNT=$((COUNT+1))\n" +
		"  echo \"$COUNT\" > \"$COUNT_FILE\"\n" +
		"  if [ \"$COUNT\" -lt 3 ]; then\n" +
		"    exit 0\n" +
		"  fi\n" +
		"  printf 'ID | Name | Host\\n123 | globular.example | 10.0.0.63\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"cluster\" ] && [ \"$2\" = \"add\" ]; then\n" +
		"  printf 'cluster added\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}

func registeredSctoolScript(logPath string) string {
	return "#!/bin/sh\n" +
		"echo \"$@\" >> " + logPath + "\n" +
		"if [ \"$1\" = \"cluster\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  printf 'ID | Name | Host\\n123 | globular.example | 10.0.0.63\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"status\" ]; then\n" +
		"  printf 'cluster healthy\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}

func preflightSctoolScript(logPath string) string {
	return "#!/bin/sh\n" +
		"echo \"$@\" >> " + logPath + "\n" +
		"if [ \"$1\" = \"version\" ]; then\n" +
		"  printf 'sctool 1.0.0\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"cluster\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
}
