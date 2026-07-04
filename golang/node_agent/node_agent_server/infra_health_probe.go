// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.infra_health_probe
// @awareness file_role=infrastructure_health_probes_gate_workflow_dispatch_and_convergence
// @awareness implements=globular.platform:intent.health.requires_fresh_evidence
// @awareness implements=globular.platform:intent.workflow.backend_health_gate_before_dispatch
// @awareness risk=high
package main

import (
	"bufio"
	"context"
	"encoding/json"
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
// Every failed attempt includes join-progress metrics (operation mode,
// streaming %, gossip peer count) in the error message so operators can
// track progress in workflow logs instead of waiting blindly for a timeout.
//
// Stuck detection: if bootstrap streaming is 100% complete but the node is
// still JOINING with no live gossip peers, the Raft topology coordinator is
// blocked by a stale dead node in system.topology — a specific error is
// returned immediately rather than waiting out the full retry window.
//
// Strategy order:
//  1. Collect join metrics via REST API (port 10000) + Prometheus (port 9180).
//  2. If stuck condition detected: return actionable error immediately.
//  3. nodetool status — "UN" → healthy.
//  4. REST API says "NORMAL" → healthy (nodetool-independent fallback).
//  5. TCP connect to CQL port 9042.
func (srv *NodeAgentServer) runProbeScyllaHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()
	if err := validateScyllaRuntimePrereqs(); err != nil {
		return probeFail(start, err.Error()), nil
	}

	localIPs := localIPv4Set()
	// Try 127.0.0.1 first (works when ScyllaDB listens on 0.0.0.0), then node IPs.
	probeAddrs := append([]string{"127.0.0.1"}, mapKeys(localIPs)...)

	// Best-effort metrics collection for rich diagnostics.
	jm := collectScyllaJoinMetrics(ctx, probeAddrs)
	if s := jm.format(); s != "" {
		log.Printf("probe-scylla-health:%s", s)
	}

	// Stuck detection: streaming 100% done but still JOINING with no live peers.
	// Cause: Raft topology coordinator waiting for a dead node still in system.topology.
	if jm.OperationMode == "JOINING" &&
		jm.BootstrapValid && jm.BootstrapPct >= 1.0 &&
		jm.GossipValid && jm.GossipLive == 0 {
		return probeFail(start,
			"ScyllaDB bootstrap streaming complete (100%) but Raft topology coordinator "+
				"is blocked: gossip_live=0 while mode=JOINING. "+
				"Likely cause: stale dead node in system.topology. "+
				"Fix: run RemoveNode for the dead host ID from a live ring member."), nil
	}

	// Strategy 1: nodetool status — "UN" (Up Normal) confirms healthy.
	if nodetool, err := exec.LookPath("nodetool"); err == nil {
		cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
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
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				status := fields[0]
				ip := fields[1]
				if localIPs[ip] {
					if scyllaSubstrateVerified(status, "") {
						return probeOK(start), nil
					}
					return probeFail(start, fmt.Sprintf("nodetool status=%s for %s%s",
						status, ip, jm.format())), nil
				}
			}
			return probeFail(start, fmt.Sprintf("nodetool ran but local IP not in ring yet%s; local IPs: %v",
				jm.format(), mapKeys(localIPs))), nil
		}
		log.Printf("probe-scylla-health: nodetool failed: %v", err)
	}

	// Strategy 2: REST API reports NORMAL — accept without nodetool.
	if scyllaSubstrateVerified("", jm.OperationMode) {
		return probeOK(start), nil
	}

	// TCP 9042 reachability is NOT health. A JOINING/BOOTSTRAPPING node can accept
	// CQL connections while not yet in the ring, so an open port must never be
	// reported as healthy/verified
	// (forbidden_fix:heuristic_signal_marks_substrate_verified). Record it only as
	// diagnostic context: verification requires Scylla substrate truth (nodetool
	// status=UN or REST operation mode=NORMAL), both of which failed above.
	cqlReachable := false
	for ip := range localIPs {
		if tryTCPConnect(ip, 9042, 2*time.Second) {
			cqlReachable = true
			break
		}
	}
	return probeFail(start, fmt.Sprintf(
		"no Scylla substrate-truth evidence: nodetool status not UN and REST mode != NORMAL "+
			"(cql_9042_reachable=%v — reachability is not health)%s", cqlReachable, jm.format())), nil
}

// scyllaSubstrateVerified is the pure health decision: a node is verified only on
// Scylla SUBSTRATE TRUTH — nodetool status "UN" (Up/Normal) or REST operation mode
// "NORMAL". Anything else (JOINING, BOOTSTRAPPING, DN, empty) is not verified.
// TCP-9042 reachability is deliberately NOT an input: an open port is not health
// (forbidden_fix:heuristic_signal_marks_substrate_verified).
func scyllaSubstrateVerified(nodetoolStatus, restOperationMode string) bool {
	return nodetoolStatus == "UN" || restOperationMode == "NORMAL"
}

// minioSubstrateVerified is the pure health decision: MinIO is verified only when
// the pool has BOTH write quorum and read quorum. Liveness (the server answers)
// and reachability (port 9000 open) are deliberately NOT inputs — a live,
// reachable server can sit on a degraded erasure set that cannot serve reads or
// writes (forbidden_fix:heuristic_signal_marks_substrate_verified).
func minioSubstrateVerified(writeQuorum, readQuorum bool) bool {
	return writeQuorum && readQuorum
}

// scyllaJoinMetrics holds a snapshot of ScyllaDB join-progress state for
// operator visibility in probe error messages.
type scyllaJoinMetrics struct {
	OperationMode  string  // "NORMAL", "JOINING", "STARTING", etc. Empty when unavailable.
	BootstrapPct   float64 // avg of scylla_streaming_finished_percentage{ops="bootstrap"} (0–1).
	BootstrapValid bool    // false when no bootstrap streaming metric was present.
	GossipLive     int     // scylla_gossip_live peer count.
	GossipValid    bool    // false when gossip metric was absent.
}

// format returns a short bracketed summary, e.g. " [mode=JOINING bootstrap=75% gossip_live=2]".
// Returns "" when no data was collected.
func (m scyllaJoinMetrics) format() string {
	if m.OperationMode == "" && !m.BootstrapValid && !m.GossipValid {
		return ""
	}
	var parts []string
	if m.OperationMode != "" {
		parts = append(parts, "mode="+m.OperationMode)
	}
	if m.BootstrapValid {
		parts = append(parts, fmt.Sprintf("bootstrap=%.0f%%", m.BootstrapPct*100))
	}
	if m.GossipValid {
		parts = append(parts, fmt.Sprintf("gossip_live=%d", m.GossipLive))
	}
	return " [" + strings.Join(parts, " ") + "]"
}

// collectScyllaJoinMetrics queries the ScyllaDB REST API (port 10000) for the
// operation mode string and the Prometheus endpoint (port 9180) for streaming
// and gossip metrics. All queries are best-effort; returns zero value on error.
func collectScyllaJoinMetrics(ctx context.Context, addrs []string) scyllaJoinMetrics {
	var m scyllaJoinMetrics
	for _, addr := range addrs {
		if mode := queryScyllaOperationModeREST(ctx, addr); mode != "" {
			m.OperationMode = mode
			break
		}
	}
	for _, addr := range addrs {
		if collectScyllaPrometheusMetrics(ctx, addr, &m) {
			break
		}
	}
	return m
}

// queryScyllaOperationModeREST calls GET /storage_service/operation_mode on the
// ScyllaDB REST API (port 10000) and returns the mode string ("NORMAL", "JOINING",
// etc.). Returns "" on any error.
func queryScyllaOperationModeREST(ctx context.Context, addr string) string {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet,
		fmt.Sprintf("http://%s:10000/storage_service/operation_mode", addr), nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var mode string
	if err := json.NewDecoder(resp.Body).Decode(&mode); err != nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(mode))
}

// collectScyllaPrometheusMetrics scrapes http://<addr>:9180/metrics and fills
// GossipLive and BootstrapPct in m. Returns true when at least one metric was found.
func collectScyllaPrometheusMetrics(ctx context.Context, addr string, m *scyllaJoinMetrics) bool {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet,
		fmt.Sprintf("http://%s:9180/metrics", addr), nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	var bootstrapSum float64
	var bootstrapCount int
	found := false

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		name, value := parsePrometheusLine(line)
		switch name {
		case "scylla_gossip_live":
			if v, err := strconv.Atoi(value); err == nil {
				m.GossipLive = v
				m.GossipValid = true
				found = true
			}
		case "scylla_streaming_finished_percentage":
			if strings.Contains(line, `ops="bootstrap"`) {
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					bootstrapSum += v
					bootstrapCount++
					found = true
				}
			}
		}
	}
	if bootstrapCount > 0 {
		m.BootstrapPct = bootstrapSum / float64(bootstrapCount)
		m.BootstrapValid = true
	}
	return found
}

// parsePrometheusLine extracts the metric name and value from a Prometheus
// text-format line. Handles both labeled (name{k=v} val) and plain (name val)
// forms. Returns empty strings when parsing fails.
func parsePrometheusLine(line string) (name, value string) {
	if idx := strings.Index(line, "{"); idx >= 0 {
		name = line[:idx]
		if end := strings.LastIndex(line, "} "); end >= 0 {
			if parts := strings.Fields(line[end+2:]); len(parts) > 0 {
				value = parts[0]
			}
		}
	} else {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name = parts[0]
			value = parts[1]
		}
	}
	return
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
		return fmt.Errorf("scylla user lookup failed: %w", err)
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

// runProbeMinioHealth checks whether MinIO is VERIFIED-healthy on this node.
//
// Verified health is SUBSTRATE TRUTH: the pool has both write quorum
// (/minio/health/cluster) and read quorum (/minio/health/cluster/read). Liveness
// (/minio/health/live) and a bare TCP connect to port 9000 prove only that the
// process answers — a MinIO server can be live and reachable while its erasure
// set is degraded and the pool cannot serve reads or writes. Reporting liveness
// or reachability as healthy is forbidden
// (forbidden_fix:heuristic_signal_marks_substrate_verified). This uses the same
// substrate-truth observer as the infra truth plane (minio_lifecycle.go).
func (srv *NodeAgentServer) runProbeMinioHealth(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()

	// Probe MinIO on this node's real interface IP, excluding the floating VIP.
	// MinIO binds to the stable node IP, not the VIP.
	vip := srv.lookupIngressVIP()
	nodeIP := config.GetLocalInterfaceIPv4(vip)
	if nodeIP == "" {
		nodeIP = config.GetRoutableIPv4()
	}

	rt := observeMinioRuntime(ctx, fmt.Sprintf("https://%s:9000", nodeIP))

	// Verified requires BOTH quorum dimensions — never liveness/reachability alone.
	if minioSubstrateVerified(rt.WriteQuorum, rt.ReadQuorum) {
		return probeOK(start), nil
	}

	// Not verified: report exactly which quorum dimension is missing, and note
	// (as diagnostic only) whether the server is merely live/reachable.
	reachable := rt.Live || tryTCPConnect(nodeIP, 9000, 2*time.Second)
	return probeFail(start, fmt.Sprintf(
		"MinIO not verified: write_quorum=%v read_quorum=%v (live=%v reachable=%v — liveness/reachability is not health); evidence: %v",
		rt.WriteQuorum, rt.ReadQuorum, rt.Live, reachable, rt.Errors)), nil
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
