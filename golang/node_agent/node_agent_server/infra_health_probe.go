package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/config"
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
	if err := validateScyllaRuntimePrereqs(); err != nil {
		return probeFail(start, err.Error()), nil
	}

	// Collect local IPs for matching nodetool output.
	localIPs := localIPv4Set()

	// --- Strategy 1: nodetool status ---
	// Pass -h <node_ip> explicitly: scylla.yaml binds api_address to the node's
	// routable IP, not 127.0.0.1 (which is nodetool's default target).
	if nodetool, err := exec.LookPath("nodetool"); err == nil {
		cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		// Pick the first local IP to use as the API host.
		var apiHost string
		for ip := range localIPs {
			apiHost = ip
			break
		}
		args := []string{"status"}
		if apiHost != "" {
			args = []string{"-h", apiHost, "status"}
		}
		cmd := exec.CommandContext(cmdCtx, nodetool, args...)
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

func validateScyllaRuntimePrereqs() error {
	// Only check files that ScyllaDB actually reads at runtime.
	// The rendered scylla.yaml uses plain CQL with no TLS — service.crt and
	// service.key are Globular gRPC certs and are NOT referenced by ScyllaDB.
	// Checking them caused a false probe failure (scylla user can't read 0400
	// gRPC key), which triggered wipe-scylla-data on otherwise healthy nodes.
	required := []string{
		"/etc/scylla/scylla.yaml",
		"/var/lib/globular/pki/ca.crt",
	}
	for _, p := range required {
		if fi, err := os.Stat(p); err != nil {
			return fmt.Errorf("scylla prereq missing/unreadable: %s (%v)", p, err)
		} else if fi.IsDir() {
			return fmt.Errorf("scylla prereq invalid: %s is a directory", p)
		}
	}
	// Verify the scylla user can read the CA cert (needed for inter-node gossip
	// trust in future TLS configurations). service.crt/.key are gRPC-only.
	scyllaUser, err := user.Lookup("scylla")
	if err != nil {
		return fmt.Errorf("scylla user lookup failed: %v", err)
	}
	uid, err := strconv.Atoi(scyllaUser.Uid)
	if err != nil {
		return fmt.Errorf("invalid scylla uid %q: %v", scyllaUser.Uid, err)
	}
	gid, err := strconv.Atoi(scyllaUser.Gid)
	if err != nil {
		return fmt.Errorf("invalid scylla gid %q: %v", scyllaUser.Gid, err)
	}
	gids := []int{gid}
	if groupIDs, err := scyllaUser.GroupIds(); err == nil {
		for _, gidStr := range groupIDs {
			if g, err := strconv.Atoi(gidStr); err == nil && g != gid {
				gids = append(gids, g)
			}
		}
	}
	if err := requireReadableByUnixUser("/var/lib/globular/pki/ca.crt", uid, gids); err != nil {
		return err
	}
	return nil
}

func requireReadableByUnixUser(path string, uid int, gids []int) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("stat metadata unavailable: %s", path)
	}
	mode := fi.Mode().Perm()
	fuid := int(st.Uid)
	fgid := int(st.Gid)
	readable := false
	switch {
	case uid == fuid && (mode&0o400) != 0:
		readable = true
	case (mode & 0o004) != 0:
		readable = true
	default:
		if (mode & 0o040) != 0 {
			for _, g := range gids {
				if g == fgid {
					readable = true
					break
				}
			}
		}
	}
	if !readable {
		return fmt.Errorf("scylla prereq unreadable by scylla user: %s (mode=%#o owner=%d:%d scylla=%d:%v)",
			filepath.Clean(path), mode, fuid, fgid, uid, gids)
	}
	return nil
}

// runProbeEtcdHealth checks whether etcd is healthy on this node.
//
// Uses the canonical config.GetEtcdClient() TLS wiring so probes do not depend
// on legacy/non-canonical certificate file paths.
func (srv *NodeAgentServer) runProbeEtcdHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()
	cli, err := config.GetEtcdClient()
	if err != nil {
		return probeFail(start, fmt.Sprintf("etcd client unavailable: %v", err)), nil
	}
	// IMPORTANT: GetEtcdClient returns a shared singleton. Do NOT close it here;
	// closing would tear down other in-flight etcd operations and create retry storms.

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Prefer probing the local node endpoint directly.
	// Use interface IP (not DNS), excluding VIP which etcd doesn't bind to.
	vip := srv.lookupIngressVIP()
	localIP := config.GetLocalInterfaceIPv4(vip)
	if localIP == "" {
		localIP = config.GetRoutableIPv4()
	}
	if localIP != "" {
		localEndpoint := fmt.Sprintf("https://%s:2379", localIP)
		if _, err := cli.Maintenance.Status(probeCtx, localEndpoint); err == nil {
			return probeOK(start), nil
		}
	}

	// Fallback: any reachable configured etcd endpoint implies etcd control
	// plane is healthy enough for cluster operations.
	var errs []string
	for _, ep := range cli.Endpoints() {
		if _, err := cli.Maintenance.Status(probeCtx, ep); err == nil {
			return probeOK(start), nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", ep, err))
		}
	}
	if len(errs) == 0 {
		return probeFail(start, "no etcd endpoints available for health probe"), nil
	}
	return probeFail(start, "etcd status failed on all endpoints: "+strings.Join(errs, "; ")), nil
}

// runProbeMinioHealth checks whether MinIO is healthy on this node.
//
// Strategy:
//  1. HTTP GET to the MinIO liveness endpoint.
//  2. Fallback: TCP connect to port 9000.
func (srv *NodeAgentServer) runProbeMinioHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()

	// Probe MinIO on this node's real interface IP, excluding the floating VIP.
	// MinIO binds to the stable node IP, not the VIP.
	vip := srv.lookupIngressVIP()
	nodeIP := config.GetLocalInterfaceIPv4(vip)
	if nodeIP == "" {
		nodeIP = config.GetRoutableIPv4()
	}

	// --- Strategy 1: HTTP liveness endpoint ---
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Get(fmt.Sprintf("https://%s:9000/minio/health/live", nodeIP))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return probeOK(start), nil
		}
		return probeFail(start, fmt.Sprintf("MinIO liveness returned HTTP %d", resp.StatusCode)), nil
	}

	// --- Strategy 2: TCP connect ---
	if tryTCPConnect(nodeIP, 9000, 2*time.Second) {
		return probeOK(start), nil
	}

	return probeFail(start, fmt.Sprintf("MinIO unreachable at %s: liveness HTTPS failed (%v) and port 9000 closed", nodeIP, err)), nil
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
