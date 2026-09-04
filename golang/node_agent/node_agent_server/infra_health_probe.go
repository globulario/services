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
	"crypto/tls"
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
	clientv3 "go.etcd.io/etcd/client/v3"
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
		// nodetool talks to ScyllaDB's admin REST API, which binds the node's
		// routable address (scylla.yaml api_address), NOT loopback — the API
		// has remote consumers. nodetool's own default IS loopback, so the
		// address has to be passed explicitly or it talks to nothing.
		//
		// scyllaAdminAPIHost picks that address deterministically. This used to
		// range over the localIPs MAP and take the first key, so on a multi-IP
		// node (LAN + docker0 + …) the choice varied run to run and the probe
		// failed intermittently with "nodetool failed: exit status 2" /
		// "std::system_error (error system:111, Connection refused)" against a
		// node whose operation_mode was NORMAL — a healthy node reported as
		// unobservable. Strategy 2's REST fallback masked it, which is why it
		// went unnoticed.
		args := []string{"status"}
		if apiHost := scyllaAdminAPIHost(localIPs); apiHost != "" {
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
					if status == "UN" {
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
	if jm.OperationMode == "NORMAL" {
		return probeOK(start), nil
	}

	// Strategy 3: TCP connect to CQL port 9042.
	for ip := range localIPs {
		if tryTCPConnect(ip, 9042, 2*time.Second) {
			return probeOK(start), nil
		}
	}

	return probeFail(start, fmt.Sprintf("CQL port 9042 unreachable and nodetool unavailable%s", jm.format())), nil
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

	probeCtx, cancel := context.WithTimeout(ctx, etcdHealthProbeBudget)
	defer cancel()

	// Probe THIS node's member, and judge only that.
	// Use interface IP (not DNS), excluding VIP which etcd doesn't bind to.
	vip := srv.lookupIngressVIP()
	localIP := config.GetLocalInterfaceIPv4(vip)
	if localIP == "" {
		localIP = config.GetRoutableIPv4()
	}
	if localIP == "" {
		// Without a local address there is no local member to judge. That is a
		// gap in the harvest, not evidence that etcd is unhealthy.
		return probeUnknown(start, "cannot determine this node's interface IP; "+
			"no local etcd member to probe"), nil
	}

	localEndpoint := fmt.Sprintf("https://%s:2379", localIP)
	localCtx, cancelLocal := context.WithTimeout(probeCtx, etcdHealthLocalAttemptBudget)
	_, localErr := cli.Maintenance.Status(localCtx, localEndpoint)
	cancelLocal()
	if localErr == nil {
		return probeOK(start), nil
	}

	// The local member did not answer. Ask the peers ONE question — "is the
	// ring up?" — because it separates the two causes this probe used to
	// conflate, and report which one we are in.
	//
	// What we must NOT do is what this code did before: treat any answering
	// peer as proof that THIS node is healthy. That reports node-3 healthy
	// because node-1 answered, which is what
	// infra.node_specific_truth_must_be_observed_via_node_local_client forbids
	// and what etcd_runtime.go's observeEtcdRuntime already gets right.
	peersAnswered, peerErrs := srv.probeEtcdPeers(probeCtx, cli, localEndpoint)
	return classifyEtcdProbe(start, localEndpoint, localErr, peersAnswered, peerErrs), nil
}

// classifyEtcdProbe turns the two observations the probe can make — did MY
// member answer, did any peer answer — into a verdict.
//
// It is a named function so the decision is testable without a live etcd; the
// alternative is a test that re-implements the rule and then agrees with
// itself. localErr is non-nil by construction here: the caller returns
// SUCCEEDED before reaching this.
func classifyEtcdProbe(start time.Time, localEndpoint string, localErr error, peersAnswered int, peerErrs []string) *node_agentpb.RunWorkflowResponse {
	// Every call this client makes fails, peers included. The client itself is
	// the most likely broken party — a node agent that cannot reach ANY member
	// while the ring serves other nodes is an unreachable observer, not an
	// unhealthy etcd (failure.an_unreachable_node_agent_is_reported_as_an_unhealthy_infra).
	// Saying UNKNOWN keeps the controller from raising infra_unhealthy on a
	// member that is fine, and keeps remediation off a component it cannot fix.
	if peersAnswered == 0 {
		return probeUnknown(start, fmt.Sprintf(
			"this node agent's etcd client reached no member, including its own (%s: %v)%s — "+
				"cannot distinguish a local etcd fault from a broken client; "+
				"check this node's service certificate before treating etcd as unhealthy",
			localEndpoint, localErr, formatPeerErrs(peerErrs)))
	}

	// Peers answer, this node's member does not: a genuine local etcd fault,
	// and the only case that earns a FAILED verdict.
	return probeFail(start, fmt.Sprintf(
		"local etcd member %s did not answer (%v) while %d peer(s) did",
		localEndpoint, localErr, peersAnswered))
}

// etcdHealthProbeBudget bounds the whole probe.
//
// etcdHealthLocalAttemptBudget bounds the LOCAL attempt specifically, so a
// stalled local member cannot consume the entire budget and leave the peer
// questions with an already-expired context. When that happened, every peer
// returned "context deadline exceeded" without a call being made, and the probe
// reported "etcd status failed on all endpoints" — five identical errors, four
// of which were never measured. That message sent the 1.2.360 soak
// investigation to the wrong component.
const (
	etcdHealthProbeBudget        = 10 * time.Second
	etcdHealthLocalAttemptBudget = 4 * time.Second
	etcdHealthPeerAttemptBudget  = 2 * time.Second
)

// probeEtcdPeers asks each non-local endpoint whether it is serving. It returns
// how many answered and the errors from those that did not. Each attempt gets
// its own budget so one unreachable peer cannot silence the rest.
func (srv *NodeAgentServer) probeEtcdPeers(ctx context.Context, cli *clientv3.Client, localEndpoint string) (int, []string) {
	var answered int
	var errs []string
	for _, ep := range cli.Endpoints() {
		if ep == localEndpoint {
			continue
		}
		if ctx.Err() != nil {
			// Out of budget. Record that we stopped asking rather than
			// recording a failure we did not observe.
			errs = append(errs, fmt.Sprintf("%s: not attempted (probe budget exhausted)", ep))
			continue
		}
		epCtx, cancel := context.WithTimeout(ctx, etcdHealthPeerAttemptBudget)
		_, err := cli.Maintenance.Status(epCtx, ep)
		cancel()
		if err == nil {
			answered++
			continue
		}
		errs = append(errs, fmt.Sprintf("%s: %v", ep, err))
	}
	return answered, errs
}

func formatPeerErrs(errs []string) string {
	if len(errs) == 0 {
		return ""
	}
	return "; peers: " + strings.Join(errs, "; ")
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

// probeUnknown reports a reduced harvest: the probe ran but could not
// establish the component's state. It is NOT a failure verdict.
//
// The controller already models this three ways (healthy / unhealthy /
// unreachable) and already says "component state UNKNOWN, not unhealthy" when
// an agent does not answer. This lets an agent that DID answer say the same
// thing about a question it could not settle, instead of being forced to pick
// between "SUCCEEDED" and a FAILED verdict it cannot support.
//
// Without it, a node agent whose own etcd client is broken reports its healthy
// local etcd as FAILED, the controller raises infra_unhealthy on that member,
// and remediation grinds against a component that was never at fault — 55
// reconcile cycles in the 1.2.360 soak run.
// See ops.always.doctor.reduced-harvest-honesty.
func probeUnknown(start time.Time, msg string) *node_agentpb.RunWorkflowResponse {
	return &node_agentpb.RunWorkflowResponse{
		Status:     node_agentpb.ProbeStatusUnknown,
		StepsTotal: 1,
		Error:      msg,
		DurationMs: time.Since(start).Milliseconds(),
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
