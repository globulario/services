package main

// scyllaGroup0Probe — first runtime vigilance probe (PR-14).
//
// It detects ScyllaDB group0 / quorum loss, the condition class the watcher
// missed because it is event-name driven. The probe SOURCES the canonical
// truth plane (node-agent GetInfraProbe for scylladb) and emits a
// DIAGNOSTIC_CLAIM interpretation. It deliberately does NOT query Scylla
// directly: group0 truth is owned by the infra-probe truth plane, and a second
// querier would be a competing source of truth — a forbidden authority bypass.
//
// Known gap, surfaced as a candidate invariant on every finding: the infra-probe
// scylladb runtime map exposes cql_ready / gossip_live / observed_peers but NOT
// an explicit group0 voter count. A group0 quorum loss that still leaves CQL
// readable (schema/topology changes blocked, reads/writes to existing keyspaces
// fine) is therefore only *inferable* from membership shrinkage + health
// violations. Making the voter count first-class on the truth plane is the
// follow-up this probe recommends.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
)

const (
	scyllaGroup0ProbeName = "scylla_group0"
	scyllaComponent       = "scylladb"
	// scyllaFoundingQuorum is the minimum ScyllaDB cluster size (HARD RULE:
	// founding quorum requires ScyllaDB on ≥3 nodes). Membership-based quorum
	// inference is only reliable at or above this size; below it we rely on
	// explicit violations to avoid false positives on dev / single-node setups.
	scyllaFoundingQuorum = 3
	// candidate AWG invariant + follow-up, recorded for human review only.
	scyllaGroup0CandidateInvariant = "scylladb.group0_voter_quorum_must_hold"
	scyllaGroup0RecommendedProbe   = "expose group0 voter count in the infra probe; cross-check with `nodetool status` and group0 history on a live member"
)

// scyllaGroup0Probe interprets the truth-plane scylladb probe for group0 health.
type scyllaGroup0Probe struct {
	srv     *server
	acquire func(ctx context.Context) (*cluster_controllerpb.InfraProbeResult, error)
}

func newScyllaGroup0Probe(srv *server) *scyllaGroup0Probe {
	p := &scyllaGroup0Probe{srv: srv}
	p.acquire = p.acquireFromTruthPlane
	return p
}

func (p *scyllaGroup0Probe) Name() string      { return scyllaGroup0ProbeName }
func (p *scyllaGroup0Probe) Component() string { return scyllaComponent }

// acquireFromTruthPlane fetches the local node-agent's scylladb infra probe.
// The node-agent handler always probes the local node, so resolving the local
// node-agent gives this node's authoritative group0 view.
func (p *scyllaGroup0Probe) acquireFromTruthPlane(ctx context.Context) (*cluster_controllerpb.InfraProbeResult, error) {
	addr := config.ResolveLocalServiceAddr("node_agent.NodeAgentService")
	if addr == "" {
		addr = config.ResolveServiceAddr("node_agent.NodeAgentService", "")
	}
	if addr == "" {
		return nil, fmt.Errorf("node_agent endpoint not resolvable")
	}
	opts, err := globular.InternalDialOptions()
	if err != nil {
		return nil, fmt.Errorf("dial options: %w", err)
	}
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial node_agent: %w", err)
	}
	defer cc.Close()
	client := node_agentpb.NewNodeAgentServiceClient(cc)
	resp, err := client.GetInfraProbe(ctx, &node_agentpb.GetInfraProbeRequest{Component: scyllaComponent})
	if err != nil {
		return nil, fmt.Errorf("GetInfraProbe: %w", err)
	}
	for _, r := range resp.GetResults() {
		if r.GetComponent() == scyllaComponent {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no %s result in infra probe response", scyllaComponent)
}

func (p *scyllaGroup0Probe) Run(ctx context.Context) ProbeResult {
	now := time.Now().Unix()
	probe, err := p.acquire(ctx)
	if err != nil {
		// A tool failure is evidence, not silence — but it is not a confirmed
		// quorum loss either. Emit an indeterminate, low-severity finding so the
		// blind spot itself is governed.
		return ProbeResult{
			ProbeName:        scyllaGroup0ProbeName,
			Component:        scyllaComponent,
			Healthy:          false,
			Indeterminate:    true,
			Condition:        "scylla_group0_probe_unavailable",
			Observed:         "node-agent infra probe unreachable: " + err.Error(),
			Expected:         "node-agent GetInfraProbe(scylladb) reachable and returning a result",
			Severity:         "warning",
			Evidence:         []string{err.Error()},
			RecommendedProbe: "check node-agent health, then run infra_probe_component scylladb (bypass_cache) on this node",
			ObservedAtUnix:   now,
		}
	}
	return interpretScyllaGroup0(probe, now)
}

// interpretScyllaGroup0 turns a truth-plane scylladb probe into a group0/quorum
// diagnostic claim. Pure function — unit-testable with a simulated probe.
func interpretScyllaGroup0(probe *cluster_controllerpb.InfraProbeResult, now int64) ProbeResult {
	res := ProbeResult{
		ProbeName:          scyllaGroup0ProbeName,
		Component:          scyllaComponent,
		EntityRef:          probe.GetNodeId() + "/" + scyllaComponent,
		TruthPlaneRef:      fmt.Sprintf("%s:%s:%d", probe.GetComponent(), probe.GetNodeId(), probe.GetProbedAtUnix()),
		ObservedAtUnix:     now,
		CandidateInvariant: scyllaGroup0CandidateInvariant,
		RecommendedProbe:   scyllaGroup0RecommendedProbe,
		Healthy:            true,
		Severity:           "info",
	}

	// Not installed here → group0 quorum is not this node's concern.
	if !probe.GetInstalled() {
		return res
	}

	rt := probe.GetRuntime()
	cqlReady := rt["cql_ready"] == "true"
	gossipLive, _ := strconv.Atoi(rt["gossip_live"])
	expectedSize := len(probe.GetExpectedPeers())
	observedMembers := len(probe.GetObservedPeers())

	evidence := []string{
		fmt.Sprintf("healthy=%t cql_ready=%t gossip_live=%d", probe.GetHealthy(), cqlReady, gossipLive),
	}
	if expectedSize > 0 {
		evidence = append(evidence, fmt.Sprintf("expected_peers=%d observed_peers=%d peers_match=%t",
			expectedSize, observedMembers, probe.GetPeersMatch()))
	}
	for _, v := range probe.GetViolations() {
		evidence = append(evidence, fmt.Sprintf("violation:%s[%s]:%s", v.GetId(), v.GetSeverity(), v.GetMessage()))
	}

	// (1) An explicit group0 / raft / quorum violation from the truth plane
	// always wins — it is the strongest signal.
	group0Violation := false
	for _, v := range probe.GetViolations() {
		hay := strings.ToLower(v.GetId() + " " + v.GetMessage())
		if strings.Contains(hay, "group0") || strings.Contains(hay, "raft") || strings.Contains(hay, "quorum") {
			group0Violation = true
			break
		}
	}

	// (2) Membership-based inference. group0 holds only while a majority of
	// expected voters are live. observed_peers lists live *peers* (excludes
	// self); self counts as live when the daemon is up or CQL is ready. Only
	// trustworthy at the founding quorum size (≥3); below that, rely on (1).
	quorumLost := false
	if expectedSize >= scyllaFoundingQuorum {
		liveMembers := observedMembers
		if probe.GetDaemonActive() || cqlReady {
			liveMembers++
		}
		if liveMembers > expectedSize {
			liveMembers = expectedSize
		}
		majority := expectedSize/2 + 1
		if liveMembers < majority {
			quorumLost = true
			evidence = append(evidence, fmt.Sprintf("live_members=%d < majority=%d of expected=%d",
				liveMembers, majority, expectedSize))
		}
	}

	switch {
	case group0Violation || quorumLost:
		res.Healthy = false
		res.Condition = "scylla_group0_quorum_loss"
		res.Severity = "critical"
		res.Observed = probe.GetSummary()
		if res.Observed == "" {
			res.Observed = "scylla group0 quorum appears lost"
		}
		res.Expected = "a majority of group0 voters live; schema/topology changes accepted"
	case !probe.GetHealthy() && probe.GetDaemonActive():
		// Member is up but not a healthy cluster member — degraded, not a
		// confirmed quorum loss. group0 may be affected; surface as a warning.
		res.Healthy = false
		res.Condition = "scylla_member_degraded"
		res.Severity = "warning"
		res.Observed = probe.GetSummary()
		res.Expected = "scylladb reports as a healthy cluster member"
	}

	res.Evidence = evidence
	return res
}
