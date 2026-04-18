//go:build integration

// Package testcluster contains integration tests that run against a live
// containerized Globular cluster (globular-quickstart).
//
// These tests NEVER run during normal `go test ./...` — the `integration`
// build tag and GLOBULAR_TEST_CLUSTER=1 guard ensure they only execute when
// explicitly invoked via `make test-integration-local` (or equivalent).
//
// Cluster assumptions (set by run-tests.sh):
//   GLOBULAR_TEST_CLUSTER=1
//   GLOBULAR_TEST_CONTAINER=globular-node-1    (docker exec target)
//   GLOBULAR_TEST_ETCD_ENDPOINT=https://10.10.0.11:2379
package testcluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// skipIfNoCluster skips the test if the environment is not set up for
// integration testing. This is a safety net — normally the build tag alone
// is sufficient, but this makes `go test -tags integration ./...` safe to
// run from any environment without an active cluster.
func skipIfNoCluster(t *testing.T) {
	t.Helper()
	if os.Getenv("GLOBULAR_TEST_CLUSTER") != "1" {
		t.Skip("GLOBULAR_TEST_CLUSTER=1 not set — skipping integration test")
	}
}

// testContainer returns the Docker container to exec into.
func testContainer() string {
	if c := os.Getenv("GLOBULAR_TEST_CONTAINER"); c != "" {
		return c
	}
	return "globular-node-1"
}

// etcdEndpoint returns the etcd endpoint for the test cluster.
func etcdEndpoint() string {
	if ep := os.Getenv("GLOBULAR_TEST_ETCD_ENDPOINT"); ep != "" {
		return ep
	}
	return "https://10.10.0.11:2379"
}

// dockerExec runs a command inside the test container and returns stdout.
func dockerExec(t *testing.T, args ...string) (string, error) {
	t.Helper()
	execArgs := append([]string{"exec", testContainer()}, args...)
	cmd := exec.Command("docker", execArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("docker exec %v: %w\nstderr: %s", args, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// etcdctl runs etcdctl inside the test container with cluster TLS credentials.
func etcdctl(t *testing.T, args ...string) (string, error) {
	t.Helper()
	base := []string{
		"etcdctl",
		"--endpoints=" + etcdEndpoint(),
		"--cacert=/var/lib/globular/pki/ca.crt",
		"--cert=/var/lib/globular/pki/issued/services/service.crt",
		"--key=/var/lib/globular/pki/issued/services/service.key",
	}
	return dockerExec(t, append(base, args...)...)
}

// etcdGet retrieves a single etcd key value.
func etcdGet(t *testing.T, key string) (string, error) {
	t.Helper()
	return etcdctl(t, "get", key, "--print-value-only")
}

// etcdPut writes a value to an etcd key.
func etcdPut(t *testing.T, key, value string) error {
	t.Helper()
	_, err := etcdctl(t, "put", key, value)
	return err
}

// waitFor polls check until it returns true or timeout expires.
func waitFor(t *testing.T, desc string, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timeout waiting for: %s (after %s)", desc, timeout)
}

// waitForKey waits until an etcd key is non-empty.
func waitForKey(t *testing.T, key string, timeout time.Duration) string {
	t.Helper()
	var val string
	waitFor(t, fmt.Sprintf("etcd key %q to be set", key), timeout, func() bool {
		v, err := etcdGet(t, key)
		if err != nil || v == "" {
			return false
		}
		val = v
		return true
	})
	return val
}

// dockerStop stops a container by name.
func dockerStop(t *testing.T, container string) {
	t.Helper()
	t.Logf("stopping container %s", container)
	cmd := exec.Command("docker", "stop", container)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker stop %s: %v\n%s", container, err, out)
	}
}

// dockerStart starts a stopped container by name.
func dockerStart(t *testing.T, container string) {
	t.Helper()
	t.Logf("starting container %s", container)
	cmd := exec.Command("docker", "start", container)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker start %s: %v\n%s", container, err, out)
	}
}

// dockerRestart restarts a container, waiting for it to be running again.
func dockerRestart(t *testing.T, container string) {
	t.Helper()
	t.Logf("restarting container %s", container)
	cmd := exec.Command("docker", "restart", container)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker restart %s: %v\n%s", container, err, out)
	}
	// Wait for container to be running
	waitFor(t, fmt.Sprintf("container %s running", container), 60*time.Second, func() bool {
		out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", container).Output()
		return err == nil && strings.TrimSpace(string(out)) == "running"
	})
	t.Logf("container %s is running", container)
}

// containerRunning returns true if the named container is in running state.
func containerRunning(container string) bool {
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", container).Output()
	return err == nil && strings.TrimSpace(string(out)) == "running"
}

// mustUnmarshalJSON parses JSON or fails the test.
func mustUnmarshalJSON(t *testing.T, data string, v interface{}) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), v); err != nil {
		t.Fatalf("unmarshal JSON: %v\ndata: %s", err, data)
	}
}

// clusterCtx returns a context with a reasonable per-test deadline.
func clusterCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)
	return ctx
}
