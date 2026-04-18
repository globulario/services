//go:build integration

package testcluster

import (
	"strings"
	"testing"
	"time"
)

// TestIntegrationSmoke verifies that the cluster is alive and all core
// services have registered. These are fast sanity checks — if any fail,
// the deeper scenario tests will also fail.
func TestIntegrationSmoke(t *testing.T) {
	skipIfNoCluster(t)

	t.Run("etcd_quorum", func(t *testing.T) {
		out, err := etcdctl(t, "endpoint", "health", "--cluster")
		if err != nil {
			t.Fatalf("etcd health check failed: %v", err)
		}
		healthy := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "is healthy") {
				healthy++
			}
		}
		if healthy < 2 {
			t.Fatalf("etcd quorum: want ≥2 healthy members, got %d\noutput: %s", healthy, out)
		}
		t.Logf("etcd: %d/3 members healthy", healthy)
	})

	t.Run("controller_elected", func(t *testing.T) {
		leader := waitForKey(t, "/globular/clustercontroller/leader", 30*time.Second)
		if leader == "" {
			t.Fatal("no cluster controller leader elected")
		}
		t.Logf("controller leader: %s", leader)
	})

	t.Run("workflow_registered", func(t *testing.T) {
		config := waitForKey(t, "/globular/services/workflow/config", 60*time.Second)
		if !strings.Contains(config, "port") {
			t.Fatalf("workflow service config missing 'port': %s", config)
		}
		t.Logf("workflow registered: %s", config)
	})

	t.Run("three_nodes_registered", func(t *testing.T) {
		waitFor(t, "≥3 nodes registered", 120*time.Second, func() bool {
			out, err := etcdctl(t, "get", "/globular/nodes/", "--prefix", "--keys-only")
			if err != nil {
				return false
			}
			count := 0
			for _, line := range strings.Split(out, "\n") {
				if strings.HasSuffix(line, "/status") {
					count++
				}
			}
			return count >= 3
		})
	})

	t.Run("desired_state_exists", func(t *testing.T) {
		// The controller should have written at least one DesiredService entry.
		out, err := etcdctl(t, "get", "/globular/resources/DesiredService/", "--prefix", "--keys-only")
		if err != nil {
			t.Fatalf("list desired services: %v", err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatal("no DesiredService entries found in etcd — controller may not have converged")
		}
		count := len(strings.Split(strings.TrimSpace(out), "\n"))
		t.Logf("desired services: %d entries", count)
	})

	t.Run("scylladb_reachable", func(t *testing.T) {
		// Check that ScyllaDB is reachable from inside node-1.
		_, err := dockerExec(t, "bash", "-c", "echo > /dev/tcp/10.10.0.20/9042")
		if err != nil {
			t.Fatalf("ScyllaDB port 9042 not reachable from node-1: %v", err)
		}
		t.Log("ScyllaDB port 9042 reachable")
	})

	t.Run("minio_reachable", func(t *testing.T) {
		// MinIO runs on node-2 (10.10.0.12:9000).
		_, err := dockerExec(t, "bash", "-c", "echo > /dev/tcp/10.10.0.12/9000")
		if err != nil {
			t.Fatalf("MinIO port 9000 not reachable from node-1: %v", err)
		}
		t.Log("MinIO port 9000 reachable")
	})
}
