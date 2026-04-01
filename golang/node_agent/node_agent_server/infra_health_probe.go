package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ---------------------------------------------------------------------------
// Infrastructure health probes
//
// These are synthetic workflows invoked via the RunWorkflow gRPC endpoint.
// Each probe checks a single infrastructure component on the local node and
// returns SUCCEEDED / FAILED with diagnostic details.
// ---------------------------------------------------------------------------

// runProbeScyllaHealth checks whether ScyllaDB is healthy on this node.
//
// Strategy:
//  1. Try `nodetool status` and look for a line containing the node's IP
//     with status "UN" (Up Normal).
//  2. Fallback: TCP connect to port 9042 on the node's interfaces.
func (srv *NodeAgentServer) runProbeScyllaHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()

	// Collect local IPs for matching nodetool output.
	localIPs := localIPv4Set()

	// --- Strategy 1: nodetool status ---
	if nodetool, err := exec.LookPath("nodetool"); err == nil {
		cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cmdCtx, nodetool, "status")
		out, err := cmd.CombinedOutput()
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := scanner.Text()
				// nodetool status lines look like:
				// UN  10.0.0.5  123.45 KiB  256  ?  rack1  datacenter1
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				status := fields[0]
				ip := fields[1]
				if localIPs[ip] {
					if status == "UN" {
						return probeOK(start), nil
					}
					return probeFail(start, fmt.Sprintf("nodetool status reports %s for %s", status, ip)), nil
				}
			}
			// nodetool ran but our IP wasn't listed — node may not be in the ring yet.
			return probeFail(start, fmt.Sprintf("nodetool status ran but local IP not found in output; local IPs: %v", mapKeys(localIPs))), nil
		}
		// nodetool failed — fall through to TCP probe.
		log.Printf("probe-scylla-health: nodetool failed: %v", err)
	}

	// --- Strategy 2: TCP connect to CQL port 9042 ---
	// ScyllaDB binds to the node's advertised IP, not 127.0.0.1.
	for ip := range localIPs {
		if tryTCPConnect(ip, 9042, 2*time.Second) {
			return probeOK(start), nil
		}
	}

	return probeFail(start, "CQL port 9042 unreachable on all local interfaces and nodetool unavailable or failed"), nil
}

// runProbeEtcdHealth checks whether etcd is healthy on this node.
//
// Runs: etcdctl endpoint health --endpoints=https://127.0.0.1:2379 ... -w json
// Expects JSON output containing {"health":true}.
func (srv *NodeAgentServer) runProbeEtcdHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()

	etcdctl, err := exec.LookPath("etcdctl")
	if err != nil {
		return probeFail(start, "etcdctl not found in PATH"), nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, etcdctl,
		"endpoint", "health",
		"--endpoints=https://127.0.0.1:2379",
		"--cert=/var/lib/globular/pki/issued/etcd/client.crt",
		"--key=/var/lib/globular/pki/issued/etcd/client.key",
		"--cacert=/var/lib/globular/pki/ca.crt",
		"-w", "json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return probeFail(start, fmt.Sprintf("etcdctl failed: %s (%v)", strings.TrimSpace(string(out)), err)), nil
	}

	// Parse JSON — etcdctl outputs an array: [{"endpoint":"...","health":true,"took":"..."}]
	var results []struct {
		Endpoint string `json:"endpoint"`
		Health   bool   `json:"health"`
	}
	if err := json.Unmarshal(out, &results); err != nil {
		return probeFail(start, fmt.Sprintf("failed to parse etcdctl output: %v; raw: %s", err, strings.TrimSpace(string(out)))), nil
	}

	for _, r := range results {
		if r.Health {
			return probeOK(start), nil
		}
	}

	return probeFail(start, fmt.Sprintf("etcd reports unhealthy: %s", strings.TrimSpace(string(out)))), nil
}

// runProbeMinioHealth checks whether MinIO is healthy on this node.
//
// Strategy:
//  1. HTTP GET to the MinIO liveness endpoint.
//  2. Fallback: TCP connect to port 9000.
func (srv *NodeAgentServer) runProbeMinioHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()

	// --- Strategy 1: HTTP liveness endpoint ---
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9000/minio/health/live")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return probeOK(start), nil
		}
		return probeFail(start, fmt.Sprintf("MinIO liveness returned HTTP %d", resp.StatusCode)), nil
	}

	// --- Strategy 2: TCP connect ---
	if tryTCPConnect("127.0.0.1", 9000, 2*time.Second) {
		return probeOK(start), nil
	}

	return probeFail(start, fmt.Sprintf("MinIO unreachable: liveness HTTP failed (%v) and port 9000 closed", err)), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func probeOK(start time.Time) *node_agentpb.RunWorkflowResponse {
	return &node_agentpb.RunWorkflowResponse{
		Status:         "SUCCEEDED",
		StepsTotal:     1,
		StepsSucceeded: 1,
		DurationMs:     time.Since(start).Milliseconds(),
	}
}

func probeFail(start time.Time, msg string) *node_agentpb.RunWorkflowResponse {
	return &node_agentpb.RunWorkflowResponse{
		Status:      "FAILED",
		StepsTotal:  1,
		StepsFailed: 1,
		Error:       msg,
		DurationMs:  time.Since(start).Milliseconds(),
	}
}

func tryTCPConnect(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// localIPv4Set returns the set of non-loopback IPv4 addresses on the machine.
func localIPv4Set() map[string]bool {
	result := make(map[string]bool)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return result
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			result[ipNet.IP.String()] = true
		}
	}
	return result
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
